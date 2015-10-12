package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/dcu/mongodb_exporter/collector"
	"github.com/dcu/mongodb_exporter/shared"
	"github.com/elephanter/nginx_exporter/nginx_export"
	"github.com/elephanter/redis_exporter/exporter"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	listeningAddress   = flag.String("telemetry.address", ":9104", "Address on which to expose metrics.")
	metricsEndpoint    = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics.")
	addr               = flag.String("addr", "unix:///var/run/docker.sock", "Docker address to connect to")
	nginxScrapeURI     = flag.String("nginx.scrape_uri", "", "URI to nginx stub status page")
	insecure           = flag.Bool("nginx.insecure", true, "Ignore server certificate if using https")
	redisAddr          = flag.String("redis.addr", "", "Address of one or more redis nodes, comma separated")
	mongodbAddr        = flag.String("mongodb.addr", "", "Mongodb URI, format: [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]")
	enabledMongoGroups = flag.String("groups.enabled", "asserts,durability,background_flushing,connections,extra_info,global_lock,index_counters,network,op_counters,op_counters_repl,memory,locks,metrics", "Comma-separated list of groups to use, for more info see: docs.mongodb.org/manual/reference/command/serverStatus/")
	parent             = flag.String("parent", "/docker", "Parent cgroup")
	authUser           = flag.String("auth.user", "", "Username for basic auth.")
	authPass           = flag.String("auth.pass", "", "Password for basic auth. Enables basic auth if set.")
	labelString        = flag.String("labels", "", "A comma seperated list of docker labels to export for containers.")
)

type basicAuthHandler struct {
	handler  http.HandlerFunc
	user     string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok || password != h.password || user != h.user {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
	return
}

func main() {
	flag.Parse()
	manager := newDockerManager(*addr, *parent)
	var labels []string
	if *labelString != "" {
		labels = strings.Split(*labelString, ",")
	} else {
		labels = make([]string, 0)
	}

	dockerClient, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatalf("Unable to start docker client %v", err.Error())
	}

	docker_exporter := NewExporter(manager, *dockerClient, labels)
	prometheus.MustRegister(docker_exporter)

	if *nginxScrapeURI != "" {
		nginx_exporter := nginx_export.NewExporter(*nginxScrapeURI, *insecure)
		prometheus.MustRegister(nginx_exporter)
	}
	
	if *redisAddr != "" {
		redisExporter := exporter.NewRedisExporter(strings.Split(*redisAddr, ","), "redis")
		prometheus.MustRegister(redisExporter)
	}

	if *mongodbAddr != "" {
		shared.LoadGroupsDesc()
		shared.ParseEnabledGroups(*enabledMongoGroups)
		mongodbCollector := collector.NewMongodbCollector(collector.MongodbCollectorOpts{
			URI: *mongodbAddr,
		})
		prometheus.MustRegister(mongodbCollector)
	}

	log.Printf("Starting Server: %s", *listeningAddress)
	handler := prometheus.Handler()
	if *authUser != "" || *authPass != "" {
		if *authUser == "" || *authPass == "" {
			glog.Fatal("You need to specify -auth.user and -auth.pass to enable basic auth")
		}
		handler = &basicAuthHandler{
			handler:  prometheus.Handler().ServeHTTP,
			user:     *authUser,
			password: *authPass,
		}
	}
	http.Handle(*metricsEndpoint, handler)
	log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
