package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

var (
	configFile = flag.String("config", "config.yml", "Configuration file name")
	logJSON    = flag.Bool("logJSON", false, "True to log in JSON format")
)

type pongConfig struct {
	Host string `yaml:"host"` // host to bind to, e.g. :8080
}

type invalidRequestError struct {
	req string
}

func (e *invalidRequestError) Error() string {
	return fmt.Sprintf("Invalid request vody: %v", e.req)
}

func main() {
	flag.Parse()

	log := logrus.New()

	if *logJSON {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{})
	}

	log.Infof("Starting Pong Service")

	var config pongConfig
	err := envconfig.Process("PONG", &config)
	if err != nil {
		log.WithError(err).Fatal("Error parsing environment configuration")
	}
	log.WithFields(logrus.Fields{
		"Host": config.Host,
	}).Info("Config loaded")

	done := make(chan struct{})

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT)

	s := http.Server{
		Addr:    config.Host,
		Handler: requestHandler(log),
	}

	go func() {
		<-sigc
		s.Shutdown(context.Background())
	}()

	go func() {
		defer close(done)

		l, err := net.Listen("tcp", config.Host)
		if err != nil {
			log.WithError(err).WithField("Host", config.Host).Fatal("Unable to listen on host")
		}

		err = s.Serve(l)
		if err == http.ErrServerClosed {
			log.Info("Shutdown gracefully")
		} else {
			log.WithError(err).Fatal("Server failed")
		}
	}()

	<-done
}

func requestHandler(log *logrus.Logger) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		reqLog := log.WithFields(logrus.Fields{
			"Method": req.Method,
			"URL":    req.URL,
			"Path":   req.URL.Path,
		})

		if strings.HasPrefix(req.URL.Path, "/ping") {
			if req.Method != http.MethodPost {
				resp.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			reqText, err := ioutil.ReadAll(req.Body)
			reqLog = reqLog.WithField("Body", string(reqText))
			if err != nil {
				resp.WriteHeader(http.StatusInternalServerError)
				reqLog.WithError(err).Error("Error reading request body")
				return
			}

			resp.WriteHeader(http.StatusOK)
			resp.Write([]byte("PONG"))

			reqLog.Info("Replied to valid request")
		} else if strings.HasPrefix(req.URL.Path, "/healthz") {
			if req.Method != http.MethodGet {
				resp.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			resp.WriteHeader(http.StatusOK)
		} else {
			resp.WriteHeader(http.StatusNotFound)
		}
	}
}
