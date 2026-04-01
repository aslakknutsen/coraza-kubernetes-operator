/*
Copyright Coraza Kubernetes Operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package main

import (
	"crypto/tls"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const envAdvValidate = "CORAZA_ADV_MANAGER_VALIDATE_SUBPROCESS"

// TestAdversarial_validateFlags_missingEnvoyClusterName validates that validateFlags
// calls os.Exit(1) when envoy-cluster-name is empty. Uses subprocess approach
// because validateFlags terminates the process directly.
func TestAdversarial_validateFlags_missingEnvoyClusterName(t *testing.T) {
	if os.Getenv(envAdvValidate) == "1" {
		validateFlags(config{})
		// If we reach here, validateFlags did NOT exit — that's wrong.
		os.Exit(2)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAdversarial_validateFlags_missingEnvoyClusterName$", "-test.v")
	cmd.Env = append(os.Environ(), envAdvValidate+"=1")
	err := cmd.Run()
	require.Error(t, err)
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "expected exit error, got %v", err)
	assert.Equal(t, 1, exitErr.ExitCode())
}

// TestAdversarial_validateFlags_withEnvoyClusterName_ok validates the success path.
func TestAdversarial_validateFlags_withEnvoyClusterName_ok(t *testing.T) {
	if os.Getenv(envAdvValidate) == "2" {
		validateFlags(config{envoyClusterName: "outbound|80||cache.default.svc.cluster.local"})
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAdversarial_validateFlags_withEnvoyClusterName_ok$", "-test.v")
	cmd.Env = append(os.Environ(), envAdvValidate+"=2")
	err := cmd.Run()
	require.NoError(t, err)
}

// TestAdversarial_buildMetricsServerOptions_allZeroValues documents behavior with a zero-value config.
func TestAdversarial_buildMetricsServerOptions_allZeroValues(t *testing.T) {
	var cfg config
	opts := buildMetricsServerOptions(cfg, nil)
	assert.Equal(t, "", opts.BindAddress)
	assert.False(t, opts.SecureServing)
	assert.Nil(t, opts.FilterProvider)
	assert.Empty(t, opts.CertDir)
}

// TestAdversarial_buildMetricsServerOptions_secureMetricsWithEmptyBindAddress documents
// "conflicting" defaults: secure metrics enabled but metrics address still empty string.
func TestAdversarial_buildMetricsServerOptions_secureMetricsWithEmptyBindAddress(t *testing.T) {
	cfg := config{
		metricsAddr:   "",
		secureMetrics: true,
	}
	opts := buildMetricsServerOptions(cfg, nil)
	assert.True(t, opts.SecureServing)
	assert.NotNil(t, opts.FilterProvider)
}

// TestAdversarial_buildMetricsServerOptions_HTTP2DisabledWithTLSOpts chains TLS opts.
func TestAdversarial_buildMetricsServerOptions_HTTP2DisabledWithTLSOpts(t *testing.T) {
	cfg := config{metricsAddr: ":9090", secureMetrics: false}
	tlsOpts := buildTLSOpts(false)
	require.Len(t, tlsOpts, 1)
	opts := buildMetricsServerOptions(cfg, tlsOpts)
	assert.Equal(t, ":9090", opts.BindAddress)
	assert.NotNil(t, opts.TLSOpts)
}

// TestAdversarial_setupWebhookServer_emptyStruct returns a non-nil server.
func TestAdversarial_setupWebhookServer_emptyStruct(t *testing.T) {
	var cfg config
	s := setupWebhookServer(cfg, nil)
	require.NotNil(t, s)
}

// TestAdversarial_setupWebhookServer_enableHTTP2FalseTLSOpts passes TLS opts.
func TestAdversarial_setupWebhookServer_enableHTTP2FalseTLSOpts(t *testing.T) {
	cfg := config{webhookCertPath: "/tmp/not-checked-in-test"}
	tlsOpts := buildTLSOpts(false)
	s := setupWebhookServer(cfg, tlsOpts)
	require.NotNil(t, s)
}

// TestAdversarial_buildTLSOpts_preservesNilForHTTP2Enabled documents that HTTP/2 returns nil.
func TestAdversarial_buildTLSOpts_preservesNilForHTTP2Enabled(t *testing.T) {
	assert.Nil(t, buildTLSOpts(true))
}

// TestAdversarial_tlsCallbackFromBuildTLSOpts_doesNotPanic applies the returned option.
func TestAdversarial_tlsCallbackFromBuildTLSOpts_doesNotPanic(t *testing.T) {
	opts := buildTLSOpts(false)
	require.Len(t, opts, 1)
	cfg := &tls.Config{}
	assert.NotPanics(t, func() { opts[0](cfg) })
	assert.Equal(t, []string{"http/1.1"}, cfg.NextProtos)
}
