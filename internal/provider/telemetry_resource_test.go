// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prashantv/gostub"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

const uuidRegex = `^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

var uuidRegexR = regexp.MustCompile(uuidRegex)

type mockServer struct {
	s     *httptest.Server
	tags  []map[string]string
	delay *time.Duration
}

func newMockServer() *mockServer {
	ms := &mockServer{}
	ms.s = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var tags map[string]string
		data, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(500)
			_, _ = writer.Write([]byte(err.Error()))
		}
		if ms.delay != nil {
			time.Sleep(*ms.delay)
		}
		_ = json.Unmarshal(data, &tags)
		ms.tags = append(ms.tags, tags)
		writer.WriteHeader(200)
	}))
	return ms
}

func newMockBlobServer(s *mockServer) *mockServer {
	ms := &mockServer{
		s: httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Write([]byte(s.serverUrl()))
		})),
	}
	return ms
}

func (ms *mockServer) close() {
	ms.s.Close()
}

func (ms *mockServer) serverUrl() string {
	return ms.s.URL
}

type stubLogger struct {
	errors []string
	traces []string
}

func (l *stubLogger) errorLog(ctx context.Context, msg string, additionalFields ...map[string]interface{}) {
	l.errors = append(l.errors, fmt.Sprintf(msg, additionalFields))
}

func (l *stubLogger) traceLog(ctx context.Context, msg string, additionalFields ...map[string]interface{}) {
	l.traces = append(l.traces, fmt.Sprintf(msg, additionalFields))
}

func TestAccTelemetryResource_endpointByEnv(t *testing.T) {
	ms := newMockServer()
	defer ms.close()
	t.Setenv("MODTM_ENDPOINT", ms.serverUrl())
	t.Run("enabled", func(t *testing.T) {
		testAccTelemetryResource(t, ms, true)
	})
	t.Run("disabled", func(t *testing.T) {
		testAccTelemetryResource(t, ms, false)
	})
}

func TestAccTelemetryResource_endpointByConfig(t *testing.T) {
	ms := newMockServer()
	defer ms.close()
	t.Run("enabled", func(t *testing.T) {
		testAccTelemetryResource(t, ms, true)
	})
	t.Run("disabled", func(t *testing.T) {
		testAccTelemetryResource(t, ms, false)
	})
}

func TestAccTelemetryResource_endpointByBlob(t *testing.T) {
	ms := newMockServer()
	defer ms.close()
	blobMs := newMockBlobServer(ms)
	defer blobMs.close()
	tags1 := map[string]string{
		"avm_git_commit":           "bc0c9fab9ee53296a64c7a682d2ed7e0726c6547",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-05-04 05:02:32",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "7634d95e-39c1-4a9a-b2e3-1fc7d6602313",
	}
	tags2 := map[string]string{
		"avm_git_commit":           "0ae8a663f1dc1dc474b14c10d9c94c77a3d1e234",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-06-05 02:21:33",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "f57d8afc-c056-4a38-b8bc-5ac303fb5737",
	}
	stub := gostub.Stub(&endpointBlobUrl, blobMs.serverUrl())
	defer stub.Reset()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags1,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags2,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
	assertEventTags(t, "create", tags1, ms)
	assertEventTags(t, "update", tags2, ms)
	assertEventTags(t, "delete", tags2, ms)
}

func TestAccTelemetryResource_endpointUnaccessableShouldFallbackToDisabledProvider(t *testing.T) {
	ms := newMockServer()
	defer ms.close()
	tags1 := map[string]string{
		"avm_git_commit":           "bc0c9fab9ee53296a64c7a682d2ed7e0726c6547",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-05-04 05:02:32",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "7634d95e-39c1-4a9a-b2e3-1fc7d6602313",
	}
	tags2 := map[string]string{
		"avm_git_commit":           "0ae8a663f1dc1dc474b14c10d9c94c77a3d1e234",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-06-05 02:21:33",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "f57d8afc-c056-4a38-b8bc-5ac303fb5737",
	}
	stub := gostub.Stub(&endpointBlobUrl, "http://") // invalid url
	defer stub.Reset()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags1,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags2,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
	assert.Empty(t, ms.tags)
}

func TestAccTelemetryResource_timeoutShouldNotBlockResource(t *testing.T) {
	ms := newMockServer()
	defer ms.close()
	logger := &stubLogger{}
	stub := gostub.Stub(&traceLog, logger.traceLog)
	stub.Stub(&errorLog, logger.errorLog)
	defer stub.Reset()
	ms.delay = &[]time.Duration{time.Second * 10}[0]
	tags := map[string]string{
		"foo": "bar",
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTelemetryResourceConfig(ms.serverUrl(), true, tags),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
		},
	})
	assert.Len(t, logger.errors, 3)
	assert.Contains(t, logger.errors[0], "timeout on create")
	assert.Contains(t, logger.errors[1], "timeout on read")
	assert.Contains(t, logger.errors[2], "timeout on delete")
}

func testAccTelemetryResource(t *testing.T, ms *mockServer, enabled bool) {
	endpoint := ms.serverUrl()
	ms.tags = make([]map[string]string, 0)
	tags1 := map[string]string{
		"avm_git_commit":           "bc0c9fab9ee53296a64c7a682d2ed7e0726c6547",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-05-04 05:02:32",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "7634d95e-39c1-4a9a-b2e3-1fc7d6602313",
	}
	tags2 := map[string]string{
		"avm_git_commit":           "0ae8a663f1dc1dc474b14c10d9c94c77a3d1e234",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-06-05 02:21:33",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "f57d8afc-c056-4a38-b8bc-5ac303fb5737",
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTelemetryResourceConfig(endpoint, enabled, tags1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags1,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig(endpoint, enabled, tags2),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags2,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
	if enabled {
		assertEventTags(t, "create", tags1, ms)
		assertEventTags(t, "update", tags2, ms)
		assertEventTags(t, "delete", tags2, ms)
	} else {
		assert.Empty(t, ms.tags)
	}
}

func resourceIdIsUuidCheck(resourceName string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(resourceName, "id", func(value string) error {
		if !uuidRegexR.Match([]byte(value)) {
			return fmt.Errorf("expect uuid as `id`, got: %s", value)
		}
		return nil
	})
}

func assertEventTags(t *testing.T, event string, tags map[string]string, s *mockServer) {
	for _, tagsReceived := range s.tags {
		if event == tagsReceived["event"] {
			resourceId := tagsReceived["resource_id"]
			assert.Regexp(t, uuidRegex, resourceId)
			delete(tagsReceived, "event")
			delete(tagsReceived, "resource_id")
			restPart := tagsReceived
			if reflect.DeepEqual(restPart, tags) {
				return
			}
		}
	}
	assert.Fail(t, `expected tags not found`, "tags are: %s", jsonMustMarshal(tags))
}

func jsonMustMarshal(m map[string]string) string {
	j, _ := json.Marshal(m)
	return string(j)
}

func testChecksForTags(res string, tags map[string]string, otherChecks ...resource.TestCheckFunc) (checks []resource.TestCheckFunc) {
	for k, v := range tags {
		checks = append(checks, resource.TestCheckResourceAttr(res, fmt.Sprintf("tags.%s", k), v))
	}
	checks = append(checks, otherChecks...)
	return
}

func testAccTelemetryResourceConfig(endpointAssignment string, enabled bool, tags map[string]string) string {
	if endpointAssignment != "" {
		endpointAssignment = fmt.Sprintf("endpoint = \"%s\"", endpointAssignment)
	}
	enabledAssignment := ""
	if !enabled {
		enabledAssignment = "enabled = false"
	}
	sb := strings.Builder{}
	for k, v := range tags {
		sb.WriteString(fmt.Sprintf("%s = \"%s\"", k, v))
		sb.WriteString("\n")
	}
	return fmt.Sprintf(`
provider "modtm" {
  %s
  %s
}

resource "modtm_telemetry" "test" {
  tags = {
   %s
  }
}
`, endpointAssignment, enabledAssignment, sb.String())
}
