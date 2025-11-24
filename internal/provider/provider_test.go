// Copyright (c) Microsoft Corporation. All rights reserved.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"modtm": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// You can add code here to run prior to any test case execution, for example assertions
	// about the appropriate environment variables being set are common to see in a pre-check
	// function.
}

func TestModuleTelemetryProvider_readEndpoint_FromProviderBlock(t *testing.T) {
	p := &ModuleTelemetryProvider{
		version: "test",
	}

	data := ModuleTelemetryProviderModel{
		Endpoint: types.StringValue("https://custom.endpoint.com/telemetry"),
	}

	ctx := context.Background()
	result := p.readEndpoint(data, ctx)

	assert.Equal(t, "https://custom.endpoint.com/telemetry", result)
	assert.False(t, p.useDefaultEndpoint, "useDefaultEndpoint should be false when endpoint is set in provider block")
}

func TestModuleTelemetryProvider_readEndpoint_FromEnvironmentVariable(t *testing.T) {
	t.Setenv("MODTM_ENDPOINT", "https://env.endpoint.com/telemetry")

	p := &ModuleTelemetryProvider{
		version: "test",
	}

	data := ModuleTelemetryProviderModel{
		Endpoint: types.StringNull(),
	}

	ctx := context.Background()
	result := p.readEndpoint(data, ctx)

	assert.Equal(t, "https://env.endpoint.com/telemetry", result)
	assert.False(t, p.useDefaultEndpoint, "useDefaultEndpoint should be false when endpoint is set via environment variable")
}

func TestModuleTelemetryProvider_readEndpoint_FromDefault(t *testing.T) {
	p := &ModuleTelemetryProvider{
		version: "test",
	}

	data := ModuleTelemetryProviderModel{
		Endpoint: types.StringNull(),
	}

	ctx := context.Background()
	result := p.readEndpoint(data, ctx)

	assert.Equal(t, defaultEndpointUrl, result)
	assert.True(t, p.useDefaultEndpoint, "useDefaultEndpoint should be true when using default endpoint")
}

func TestModuleTelemetryProvider_readEndpoint_PriorityOrder(t *testing.T) {
	// Test that provider block takes priority over environment variable
	t.Setenv("MODTM_ENDPOINT", "https://env.endpoint.com/telemetry")

	p := &ModuleTelemetryProvider{
		version: "test",
	}

	data := ModuleTelemetryProviderModel{
		Endpoint: types.StringValue("https://provider.endpoint.com/telemetry"),
	}

	ctx := context.Background()
	result := p.readEndpoint(data, ctx)

	assert.Equal(t, "https://provider.endpoint.com/telemetry", result, "provider block endpoint should take priority")
	assert.False(t, p.useDefaultEndpoint, "useDefaultEndpoint should be false when endpoint is set in provider block")
}

func TestCheckAndFollowRedirect_NoRedirect(t *testing.T) {
	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := checkAndFollowRedirect(server.URL)
	assert.Equal(t, server.URL, result, "should return original URL when no redirect")
}

func TestCheckAndFollowRedirect_Single301(t *testing.T) {
	// Create a test server that returns 301 redirect
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", finalServer.URL)
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	result := checkAndFollowRedirect(redirectServer.URL)
	assert.Equal(t, finalServer.URL, result, "should follow 301 redirect")
}

func TestCheckAndFollowRedirect_Multiple301(t *testing.T) {
	// Create a chain of redirects
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer finalServer.Close()

	redirect2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", finalServer.URL)
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer redirect2.Close()

	redirect1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", redirect2.URL)
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer redirect1.Close()

	result := checkAndFollowRedirect(redirect1.URL)
	assert.Equal(t, finalServer.URL, result, "should follow multiple 301 redirects")
}

func TestCheckAndFollowRedirect_EmptyEndpoint(t *testing.T) {
	result := checkAndFollowRedirect("")
	assert.Equal(t, "", result, "should return empty string for empty endpoint")
}

func TestCheckAndFollowRedirect_InvalidURL(t *testing.T) {
	result := checkAndFollowRedirect("http://invalid-url-that-does-not-exist-12345.com")
	assert.Equal(t, "http://invalid-url-that-does-not-exist-12345.com", result, "should return original URL on error")
}

func TestCheckAndFollowRedirect_Non301StatusCode(t *testing.T) {
	// Test various non-301 status codes
	statusCodes := []int{200, 302, 404, 500}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			result := checkAndFollowRedirect(server.URL)
			assert.Equal(t, server.URL, result, "should return original URL for status code %d", code)
		})
	}
}

func TestCheckAndFollowRedirect_MaxDepth(t *testing.T) {
	// Create a server that always redirects to itself (infinite loop)
	var redirectServer *httptest.Server
	redirectServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", redirectServer.URL+"/next")
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	result := checkAndFollowRedirect(redirectServer.URL)
	// Should stop after max depth (10) and return the last attempted URL
	assert.NotEmpty(t, result, "should return a URL even after hitting max depth")
}

func TestCheckAndFollowRedirect_Timeout(t *testing.T) {
	// Stub the timeout to be very short for testing
	stub := gostub.Stub(&readDefaultEndpointTimeout, 10*time.Millisecond)
	defer stub.Reset()

	// Create a server that delays response longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	done := make(chan string)
	go func() {
		result := checkAndFollowRedirect(server.URL)
		done <- result
	}()

	select {
	case result := <-done:
		// Should return original URL on timeout
		assert.Equal(t, server.URL, result, "should return original URL on timeout")
	case <-time.After(15 * time.Millisecond):
		t.Fatal("checkAndFollowRedirect should complete within timeout + buffer")
	}
}
