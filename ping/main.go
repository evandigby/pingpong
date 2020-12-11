package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

var (
	logJSON = flag.Bool("logJSON", false, "True to log in JSON format")
)

type pingConfig struct {
	PongURL  string        `yaml:"pongURL"`  // Host:Port for Pong Service
	Interval time.Duration `yaml:"interval"` // Time between pings
	Timeout  time.Duration `yaml:"timeout"`  // Ping request timeout
}

type invalidStatusError struct {
	statusCode int
	status     string
}

func (e *invalidStatusError) Error() string {
	return fmt.Sprintf("Invalid status code %v (%v)", e.statusCode, e.status)
}

type invalidResponseError struct {
	resp string
}

func (e *invalidResponseError) Error() string {
	return fmt.Sprintf("Invalid response: %v", e.resp)
}

func main() {
	flag.Parse()

	log := logrus.New()

	if *logJSON {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{})
	}

	log.Infof("Starting Ping Service")

	var config pingConfig
	err := envconfig.Process("PING", &config)
	if err != nil {
		log.WithError(err).Fatal("Error parsing environment configuration")
	}
	log.WithFields(logrus.Fields{
		"Pong URL": config.PongURL,
		"Interval": config.Interval,
		"Timeout":  config.Timeout,
	}).Info("Config loaded")

	done := make(chan struct{})

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT)

	go func() {
		defer close(done)

		t := time.NewTicker(config.Interval)
		for {
			select {
			case <-t.C:
				err := sendPing(config.PongURL, config.Timeout)
				if err != nil {
					errLog := log.WithError(err).WithField("URL", config.PongURL)
					if statusErr, ok := err.(*invalidStatusError); ok {
						errLog = errLog.WithFields(logrus.Fields{
							"Status":      statusErr.status,
							"Status Code": statusErr.statusCode,
						})
					}

					errLog.Error("Unable to contact PONG url")
					continue
				}

				log.Info("Sent ping")
			case <-sigc:
				// Exit the program
				return
			}
		}
	}()

	<-done
}

func sendPing(
	pongURL string,
	timeout time.Duration,
) error {

	ctx := context.Background()
	var cancel context.CancelFunc = func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}

	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, pongURL, strings.NewReader("PING"))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return &invalidStatusError{statusCode: resp.StatusCode, status: resp.Status}
	}

	respText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if string(respText) != "PONG" {
		return &invalidResponseError{resp: string(respText)}
	}

	return nil
}
