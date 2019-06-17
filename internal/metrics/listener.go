/*
 * Unless explicitly stated otherwise all files in this repository are licensed
 * under the Apache License Version 2.0.
 *
 * This product includes software developed at Datadog (https://www.datadoghq.com/).
 * Copyright 2019 Datadog, Inc.
 */

package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DataDog/datadog-lambda-go/internal/logger"
)

type (
	// Listener implements wrapper.HandlerListener, injecting metrics into the context
	Listener struct {
		apiClient *APIClient
		config    *Config
	}

	// Config gives options for how the listener should work
	Config struct {
		APIKey               string
		KMSAPIKey            string
		Site                 string
		ShouldRetryOnFailure bool
		BatchInterval        time.Duration
	}
)

// MakeListener initializes a new metrics lambda listener
func MakeListener(config Config) Listener {

	site := config.Site
	if site == "" {
		site = defaultSite
	}
	baseAPIURL := fmt.Sprintf("https://api.%s/api/v1", site)

	apiClient := MakeAPIClient(context.Background(), APIClientOptions{
		baseAPIURL: baseAPIURL,
		apiKey:     config.APIKey,
		decrypter:  MakeKMSDecrypter(),
		kmsAPIKey:  config.KMSAPIKey,
	})
	if config.BatchInterval <= 0 {
		config.BatchInterval = defaultBatchInterval
	}

	return Listener{
		apiClient,
		&config,
	}
}

// HandlerStarted adds metrics service to the context
func (l *Listener) HandlerStarted(ctx context.Context, msg json.RawMessage) context.Context {
	if l.apiClient.apiKey == "" {
		logger.Error(fmt.Errorf("datadog api key isn't set, won't be able to send metrics"))
	}

	ts := MakeTimeService()
	pr := MakeProcessor(ctx, l.apiClient, ts, l.config.BatchInterval, l.config.ShouldRetryOnFailure)

	ctx = AddProcessor(ctx, pr)
	// Setting the context on the client will mean that future requests will be cancelled correctly
	// if the lambda times out.
	l.apiClient.context = ctx

	pr.StartProcessing()

	return ctx
}

// HandlerFinished implemented as part of the wrapper.HandlerListener interface
func (l *Listener) HandlerFinished(ctx context.Context) {
	pr := GetProcessor(ctx)
	if pr != nil {
		pr.FinishProcessing()
	}
}
