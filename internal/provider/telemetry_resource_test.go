// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

var ms *mockServer

const uuidRegex = `^[0-9a-fA-F]{8}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{4}\b-[0-9a-fA-F]{12}$`

var uuidRegexR = regexp.MustCompile(uuidRegex)

type mockServer struct {
	s    *httptest.Server
	tags []map[string]string
}

func NewMockServer() *mockServer {
	ms := &mockServer{}
	ms.s = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var tags map[string]string
		data, err := io.ReadAll(request.Body)
		if err != nil {
			writer.WriteHeader(500)
			_, _ = writer.Write([]byte(err.Error()))
		}
		_ = json.Unmarshal(data, &tags)
		ms.tags = append(ms.tags, tags)
	}))
	return ms
}

func (ms *mockServer) Close() {
	ms.s.Close()
}

func (ms *mockServer) ServerUrl() string {
	return ms.s.URL
}

func TestMain(m *testing.M) {
	ms = NewMockServer()
	defer ms.Close()
	m.Run()
}

func TestAccTelemetryResource_endpointByEnv(t *testing.T) {
	t.Setenv("MODTM_ENDPOINT", ms.ServerUrl())
	testAccTelemetryResource(t, "")
}

func TestAccTelemetryResource_endpointByConfig(t *testing.T) {
	testAccTelemetryResource(t, fmt.Sprintf("endpoint = \"%s\"", ms.ServerUrl()))
}

func testAccTelemetryResource(t *testing.T, endpoint string) {
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
				Config: testAccTelemetryResourceConfig(endpoint, tags1),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(
						"modtm_telemetry.test", tags1,
						resourceIdIsUuidCheck("modtm_telemetry.test"),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig(endpoint, tags2),
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
	assertEventTags(t, "create", tags1)
	assertEventTags(t, "update", tags2)
	assertEventTags(t, "delete", tags2)
}

func resourceIdIsUuidCheck(resourceName string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(resourceName, "id", func(value string) error {
		if !uuidRegexR.Match([]byte(value)) {
			return fmt.Errorf("expect uuid as `id`, got: %s", value)
		}
		return nil
	})
}

func assertEventTags(t *testing.T, event string, tags map[string]string) {
	for _, tagsReceived := range ms.tags {
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

func testAccTelemetryResourceConfig(endpoint string, tags map[string]string) string {
	sb := strings.Builder{}
	for k, v := range tags {
		sb.WriteString(fmt.Sprintf("%s = \"%s\"", k, v))
		sb.WriteString("\n")
	}
	return fmt.Sprintf(`
provider "modtm" {
  %s
}

resource "modtm_telemetry" "test" {
  tags = {
   %s
  }
}
`, endpoint, sb.String())
}
