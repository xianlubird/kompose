package compose

import (
	"fmt"
	"github.com/flynn/go-shlex"
	"github.com/kubernetes/kompose/pkg/kobject"
	"github.com/pkg/errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Support Docker Commander Special Label
const EXT_LABEL_PREFIX = "aliyun"
const LABEL_SERVICE_SCALE = EXT_LABEL_PREFIX + ".scale"
const LABEL_SERVICE_LOGS = EXT_LABEL_PREFIX + ".logs"
const LABEL_SERVICE_GLOBAL = EXT_LABEL_PREFIX + ".global"
const LABEL_SERVICE_DEPENDS = EXT_LABEL_PREFIX + ".depends"

const LABEL_SERVICE_ROUTING_SEPARATOR = ","
const LABEL_SERVICE_ROUTING_PREFIX = EXT_LABEL_PREFIX + ".routing"
const LABEL_SERVICE_ROUTING_SESSION = LABEL_SERVICE_ROUTING_PREFIX + ".session_sticky"
const LABEL_SERVICE_ROUTING_WEIGHT = LABEL_SERVICE_ROUTING_PREFIX + ".weight"
const LABEL_SERVICE_ROUTING_SSL_CERTS = LABEL_SERVICE_ROUTING_PREFIX + ".ssl_cert"
const LABEL_SERVICE_ROUTING_EXTRA_SETTINGS = LABEL_SERVICE_ROUTING_PREFIX + ".extra_setting"
const LABEL_SERVICE_ROUTING_TIMEOUT_SETTINGS = LABEL_SERVICE_ROUTING_PREFIX + ".timeout_setting"
const LABEL_SERVICE_ROUTING_WORKER_NUMBER = LABEL_SERVICE_ROUTING_PREFIX + ".worker"
const LABEL_SERVICE_ROUTING_PORT_PREFIX = LABEL_SERVICE_ROUTING_PREFIX + ".port_"

const LABEL_SERVICE_LB_PORT_PREFIX = EXT_LABEL_PREFIX + ".lb.port_"
const LABEL_SERVICE_VIP_PORT_PREFIX = EXT_LABEL_PREFIX + ".vip.port_"

// Probe Labels
const LABEL_SERVICE_PROBE_URL = EXT_LABEL_PREFIX + ".probe.url"
const LABEL_SERVICE_PROBE_CMD = EXT_LABEL_PREFIX + ".probe.cmd"
const LABEL_SERVICE_PROBE_INIT_DELAY_SECONDS = EXT_LABEL_PREFIX + ".probe.initial_delay_seconds"
const LABEL_SERVICE_PROBE_TIMEOUT_SECONDS = EXT_LABEL_PREFIX + ".probe.timeout_seconds"

// sls labels
const LABEL_SLS_PREFIX = EXT_LABEL_PREFIX + ".log_store_"
const LABEL_SLS_PROJECT = EXT_LABEL_PREFIX + ".log_project"
const LABEL_SLS_TTL_PATTERN = EXT_LABEL_PREFIX + ".log_ttl_%s"

// monitoring labels
const LABEL_MONITORING_PREFIX = EXT_LABEL_PREFIX + ".monitoring"
const LABEL_MONITORING_CONF = LABEL_MONITORING_PREFIX + ".conf"
const LABEL_MONITORING_HTTP = LABEL_MONITORING_PREFIX + ".http"
const LABEL_MONITORING_SCRIPT = LABEL_MONITORING_PREFIX + ".script"
const LABEL_MONITORING_RELOAD = LABEL_MONITORING_PREFIX + ".reload"
const LABEL_MONITORING_TAGS = LABEL_MONITORING_PREFIX + ".tags"
const LABEL_MONITORING_FORMAT = LABEL_MONITORING_PREFIX + ".format"
const LABEL_MONITORING_INTERVAL = LABEL_MONITORING_PREFIX + ".interval"

const LABEL_MONITORING_ADDON_PREFIX = LABEL_MONITORING_PREFIX + ".addon"
const LABEL_MONITORING_ADDON_INFLUXDB = LABEL_MONITORING_ADDON_PREFIX + ".influxdb"
const LABEL_MONITORING_ADDON_INFLUXDB_RETENTION_POLICY = LABEL_MONITORING_ADDON_PREFIX + ".influxdb_retention_policy"
const LABEL_MONITORING_ADDON_PROMETHEUS = LABEL_MONITORING_ADDON_PREFIX + ".prometheus"

// auto-scaling labels
const LABEL_AUTO_SCALING_PREFIX = EXT_LABEL_PREFIX + ".auto_scaling"
const LABEL_AUTO_SCALING_MAX_CPU = LABEL_AUTO_SCALING_PREFIX + ".max_cpu"
const LABEL_AUTO_SCALING_MIN_CPU = LABEL_AUTO_SCALING_PREFIX + ".min_cpu"
const LABEL_AUTO_SCALING_MAX_MEMORY = LABEL_AUTO_SCALING_PREFIX + ".max_memory"
const LABEL_AUTO_SCALING_MIN_MEMORY = LABEL_AUTO_SCALING_PREFIX + ".min_memory"
const LABEL_AUTO_SCALING_MAX_INTERNET_INRATE = LABEL_AUTO_SCALING_PREFIX + ".max_internetInRate"
const LABEL_AUTO_SCALING_MIN_INTERNET_INRATE = LABEL_AUTO_SCALING_PREFIX + ".min_internetInRate"
const LABEL_AUTO_SCALING_MAX_INTERNET_OUTRATE = LABEL_AUTO_SCALING_PREFIX + ".max_internetOutRate"
const LABEL_AUTO_SCALING_MIN_INTERNET_OUTRATE = LABEL_AUTO_SCALING_PREFIX + ".min_internetOutRate"

const LABEL_AUTO_SCALING_MAX_INSTANCES = LABEL_AUTO_SCALING_PREFIX + ".max_instances"
const LABEL_AUTO_SCALING_MIN_INSTANCES = LABEL_AUTO_SCALING_PREFIX + ".min_instances"
const LABEL_AUTO_SCALING_STEP = LABEL_AUTO_SCALING_PREFIX + ".step"
const LABEL_AUTO_SCALING_PERIOD = LABEL_AUTO_SCALING_PREFIX + ".period"

// This function will retrieve volumes for each service, as well as it will parse volume information and store it in Volumes struct
func handleAliyunExt(svcName string, service *kobject.ServiceConfig) {
	for key, value := range service.Labels {

		value = strings.TrimSpace(value)
		var err error

		switch key {

		case LABEL_SERVICE_SCALE:
			service.Replicas, err = strconv.Atoi(value)
		case LABEL_SERVICE_GLOBAL:
			if strings.ToLower(value) == "true" {
				service.DeployMode = "global"
			}
		case LABEL_SERVICE_PROBE_URL:
			err = parseServiceProbe(value, &service.HealthChecks)
		case LABEL_SERVICE_PROBE_CMD:
			service.HealthChecks.Test, err = shlex.Split(value)
		case LABEL_SERVICE_PROBE_TIMEOUT_SECONDS:
			service.HealthChecks.Timeout, err = parseProbeSeconds(value)
		case LABEL_SERVICE_PROBE_INIT_DELAY_SECONDS:
			service.HealthChecks.StartPeriod, err = parseProbeSeconds(value)
		}

		if err != nil {
			errors.Wrap(err, fmt.Sprint("invalid value for %s label: %s", key, value))
		}
	}
}

const _CONTAINER = "container"

func parseServiceProbe(value string, probe *kobject.HealthCheck) error {

	var err error
	uri, err := url.Parse(value)
	if err != nil {
		return err
	}
	switch uri.Scheme {
	case "http":
		host := uri.Host
		portStr := "80"
		port := 80
		if strings.Index(uri.Host, ":") >= 0 {
			host, portStr, err = net.SplitHostPort(uri.Host)
		}
		if portStr != "" {
			port, err = strconv.Atoi(portStr)
		}
		if err == nil {
			if host != _CONTAINER {
				err = fmt.Errorf("Host of HTTP probe URL should be container")
			} else {
				action := kobject.HTTPGetAction{
					Port: port,
					Path: uri.Path,
				}
				probe.HTTPGet = &action
			}
		}
	case "tcp":
		host, portStr, err2 := net.SplitHostPort(uri.Host)
		err := err2
		port := 0
		if portStr == "" {
			err = fmt.Errorf("Host of HTTP probe URL should be container")
		} else if host != _CONTAINER {
			err = fmt.Errorf("Host of TCP probe should be container")
		} else {
			port, err = strconv.Atoi(portStr)
		}
		if err == nil {
			action := kobject.TCPSocketAction{
				Port: port,
			}
			probe.TCPSocket = &action
		}
	default:
		err = fmt.Errorf("Unsupported scheme of probe URL: %s", value)
	}
	return err
}

func parseProbeSeconds(value string) (int32, error) {
	result, err := strconv.Atoi(value)
	return int32(result), err
}
