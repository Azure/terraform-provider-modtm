// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/toxiproxy/v2/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
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
			_, _ = writer.Write([]byte(s.serverUrl()))
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

type accTelemetryResourceSuite struct {
	suite.Suite
}

func TestAccTelemetryResource(t *testing.T) {
	suite.Run(t, new(accTelemetryResourceSuite))
}

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_endpointByEnv() {
	ms := newMockServer()
	defer ms.close()
	s.T().Setenv("MODTM_ENDPOINT", ms.serverUrl())
	s.Run("enabled", func() {
		testAccTelemetryResource(s.T(), ms, true)
	})
	s.Run("disabled", func() {
		testAccTelemetryResource(s.T(), ms, false)
	})
}

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_endpointByConfig() {
	ms := newMockServer()
	defer ms.close()
	s.Run("enabled", func() {
		testAccTelemetryResource(s.T(), ms, true)
	})
	s.Run("disabled", func() {
		testAccTelemetryResource(s.T(), ms, false)
	})
}

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_endpointByBlob() {
	t := s.T()
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
				Config: testAccTelemetryResourceConfig("", true, tags1, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags1,
						resourceIdIsUuidCheck(),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags2, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags2,
						resourceIdIsUuidCheck(),
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

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_updateNonce() {
	t := s.T()
	ms := newMockServer()
	defer ms.close()
	blobMs := newMockBlobServer(ms)
	defer blobMs.close()
	tags := map[string]string{
		"avm_git_commit":           "bc0c9fab9ee53296a64c7a682d2ed7e0726c6547",
		"avm_git_file":             "main.tf",
		"avm_git_last_modified_at": "2023-05-04 05:02:32",
		"avm_git_org":              "Azure",
		"avm_git_repo":             "terraform-azurerm-aks",
		"avm_yor_trace":            "7634d95e-39c1-4a9a-b2e3-1fc7d6602313",
	}
	stub := gostub.Stub(&endpointBlobUrl, blobMs.serverUrl())
	defer stub.Reset()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("modtm_telemetry.test", "nonce", "0")),
			},
			{
				Config: testAccTelemetryResourceConfig("", true, tags, intPtr(1)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("modtm_telemetry.test", "nonce", "1")),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags, intPtr(2)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("modtm_telemetry.test", "nonce", "2")),
			},
			{
				Config: testAccTelemetryResourceConfig("", true, tags, intPtr(1)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("modtm_telemetry.test", "nonce", "1")),
			},
			{
				Config: testAccTelemetryResourceConfig("", true, tags, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("modtm_telemetry.test", "nonce", "1")), // Remove `nonce` from the config won't remove it from the state
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_endpointUnaccessableShouldFallbackToDisabledProvider() {
	t := s.T()
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
				Config: testAccTelemetryResourceConfig("", true, tags1, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags1,
						resourceIdIsUuidCheck(),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig("", true, tags2, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags2,
						resourceIdIsUuidCheck(),
					)...,
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
	assert.Empty(t, ms.tags)
}

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_timeoutShouldNotBlockResource() {
	t := s.T()
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
				Config: testAccTelemetryResourceConfig(ms.serverUrl(), true, tags, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags,
						resourceIdIsUuidCheck(),
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

func (s *accTelemetryResourceSuite) TestAccTelemetryResource_ResourceEndpoint() {
	t := s.T()
	cases := []struct {
		desc   string
		config string
	}{
		{
			desc: "resource_endpoint_literals",
			config: `
resource "modtm_telemetry" "test" {
  tags = {
   %[1]s
  }
  endpoint = "%[2]s"
}
`,
		},
		{
			desc: "resource_endpoint_reference",
			config: `
locals {
	endpoint = "%[2]s"
}

resource "modtm_telemetry" "test" {
  tags = {
   %[1]s
  }
  endpoint = local.endpoint
}
`,
		},
	}
	for _, c := range cases {
		s.Run(c.desc, func() {
			providerEndpointServer := newMockServer()
			defer providerEndpointServer.close()
			blobMs := newMockBlobServer(providerEndpointServer)
			defer blobMs.close()
			resourceEndpointServer := newMockServer()
			defer resourceEndpointServer.close()
			tags := map[string]string{
				"avm_git_commit":           "bc0c9fab9ee53296a64c7a682d2ed7e0726c6547",
				"avm_git_file":             "main.tf",
				"avm_git_last_modified_at": "2023-05-04 05:02:32",
				"avm_git_org":              "Azure",
				"avm_git_repo":             "terraform-azurerm-aks",
				"avm_yor_trace":            "7634d95e-39c1-4a9a-b2e3-1fc7d6602313",
			}
			tagsBuilder := strings.Builder{}
			for k, v := range tags {
				tagsBuilder.WriteString(fmt.Sprintf("%s = \"%s\"", k, v))
				tagsBuilder.WriteString("\n")
			}
			stub := gostub.Stub(&endpointBlobUrl, blobMs.serverUrl())
			defer stub.Reset()
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					// Create and Read testing
					{
						Config: fmt.Sprintf(c.config, tagsBuilder.String(), resourceEndpointServer.serverUrl()),
						Check: resource.ComposeAggregateTestCheckFunc(
							testChecksForTags(tags,
								resourceIdIsUuidCheck(),
							)...,
						),
					},
				},
			})
			s.Empty(providerEndpointServer.tags)
			assertEventTags(t, "create", tags, resourceEndpointServer)
			assertEventTags(t, "delete", tags, resourceEndpointServer)
		})
	}
}

type ChaosTestSuite struct {
	suite.Suite
	ms         *mockServer
	toxiClient *toxiproxy.Client
	toxi       *toxiproxy.Proxy
}

func TestChaosTelemetryResource(t *testing.T) {
	if chaos := os.Getenv("CHAOS"); chaos == "" {
		t.Skip("chaos tests only run when there's `CHAOS` environment variable.")
	}
	suite.Run(t, new(ChaosTestSuite))
}

func (s *ChaosTestSuite) SetupSuite() {
	s.ms = newMockServer()
	client := toxiproxy.NewClient("localhost:8474")
	s.toxiClient = client
	randomPort, err := getRandomPort()
	if err != nil {
		panic("cannot allocate a free random port")
	}
	s.toxi, err = client.CreateProxy("mockServer", fmt.Sprintf("localhost:%d", randomPort), strings.TrimPrefix(s.ms.serverUrl(), "http://"))
	if err != nil {
		panic(fmt.Errorf("cannot create toxiproxy client: %s", err.Error()))
	}
}

func (s *ChaosTestSuite) TearDownSuite() {
	_ = s.toxi.Delete()
	_ = s.toxiClient.ResetState()
	s.ms.close()
}

func (s *ChaosTestSuite) TestChaosTelemetryResource_ServerDown() {
	if chaos := os.Getenv("CHAOS"); chaos == "" {
		s.T().Skip("chaos tests only run when there's `CHAOS` environment variable.")
	}

	if err := s.toxi.Disable(); err != nil {
		s.FailNowf(`cannot setup toxiproxy: %s`, err.Error())
	}
	defer func() {
		_ = s.toxi.Enable()
	}()

	timeoutErr := runWithTimeout(time.Second*10, func() {
		testTelemetryResource(s.T(), fmt.Sprintf("http://%s", s.toxi.Listen), true)
	})
	assert.NoError(s.T(), timeoutErr)
}

func (s *ChaosTestSuite) TestChaosTelemetryResource_Latency_NoTimeout() {
	if chaos := os.Getenv("CHAOS"); chaos == "" {
		s.T().Skip("chaos tests only run when there's `CHAOS` environment variable.")
	}

	toxic, err := s.toxi.AddToxic("latency", "latency", "upstream", 1.0, toxiproxy.Attributes{
		"latency": 1000,
	})
	if err != nil {
		s.FailNowf(`cannot setup toxiproxy: %s`, err.Error())
	}
	defer func() {
		_ = s.toxi.RemoveToxic(toxic.Name)
	}()

	timeoutErr := runWithTimeout(time.Second*10, func() {
		testTelemetryResource(s.T(), fmt.Sprintf("http://%s", s.toxi.Listen), true)
	})
	assert.NoError(s.T(), timeoutErr)
}

func (s *ChaosTestSuite) TestChaosTelemetryResource_Latency_Timeout() {
	if chaos := os.Getenv("CHAOS"); chaos == "" {
		s.T().Skip("chaos tests only run when there's `CHAOS` environment variable.")
	}

	toxic, err := s.toxi.AddToxic("latency", "latency", "upstream", 1.0, toxiproxy.Attributes{
		"latency": 5000,
	})
	if err != nil {
		s.FailNowf(`cannot setup toxiproxy: %s`, err.Error())
	}
	defer func() {
		_ = s.toxi.RemoveToxic(toxic.Name)
	}()

	// The test would call create, update, delete, and each operation would cause a read, so the total time should exceed 5*6=30 secs
	timeoutErr := runWithTimeout(time.Second*35, func() {
		testTelemetryResource(s.T(), fmt.Sprintf("http://%s", s.toxi.Listen), true)
	})
	assert.NoError(s.T(), timeoutErr)
}

func (s *ChaosTestSuite) TestChaosTelemetryResource_ResetPeer() {
	if chaos := os.Getenv("CHAOS"); chaos == "" {
		s.T().Skip("chaos tests only run when there's `CHAOS` environment variable.")
	}

	toxic, err := s.toxi.AddToxic("reset_peer", "reset_peer", "upstream", 1.0, toxiproxy.Attributes{})
	if err != nil {
		s.FailNowf(`cannot setup toxiproxy: %s`, err.Error())
	}
	defer func() {
		_ = s.toxi.RemoveToxic(toxic.Name)
	}()

	// The test would call create, update, delete, and each operation would cause a read, so the total time should exceed 5*6=30 secs
	timeoutErr := runWithTimeout(time.Second*5, func() {
		testTelemetryResource(s.T(), fmt.Sprintf("http://%s", s.toxi.Listen), true)
	})
	assert.NoError(s.T(), timeoutErr)
}

func runWithTimeout(timeout time.Duration, callback func()) error {
	done := make(chan struct{})
	go func() {
		callback()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("operation timed out")
	}
}

func getRandomPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = l.Close()
	}()
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("cannot allocate a random tcp port")
	}
	return tcpAddr.Port, nil
}

func testAccTelemetryResource(t *testing.T, ms *mockServer, enabled bool) {
	endpoint := ms.serverUrl()
	ms.tags = make([]map[string]string, 0)
	tags1, tags2 := testTelemetryResource(t, endpoint, enabled)
	if enabled {
		assertEventTags(t, "create", tags1, ms)
		assertEventTags(t, "update", tags2, ms)
		assertEventTags(t, "delete", tags2, ms)
	} else {
		assert.Empty(t, ms.tags)
	}
}

func testTelemetryResource(t *testing.T, endpoint string, enabled bool) (map[string]string, map[string]string) {
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
				Config: testAccTelemetryResourceConfig(endpoint, enabled, tags1, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags1,
						resourceIdIsUuidCheck(),
					)...,
				),
			},
			// Update and Read testing
			{
				Config: testAccTelemetryResourceConfig(endpoint, enabled, tags2, nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					testChecksForTags(tags2,
						resourceIdIsUuidCheck(),
					)...,
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
	return tags1, tags2
}

func resourceIdIsUuidCheck() resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith("modtm_telemetry.test", "id", func(value string) error {
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

func testChecksForTags(tags map[string]string, otherChecks ...resource.TestCheckFunc) (checks []resource.TestCheckFunc) {
	for k, v := range tags {
		checks = append(checks, resource.TestCheckResourceAttr("modtm_telemetry.test", fmt.Sprintf("tags.%s", k), v))
	}
	checks = append(checks, otherChecks...)
	return
}

func testAccTelemetryResourceConfig(endpointAssignment string, enabled bool, tags map[string]string, nonce *int) string {
	if endpointAssignment != "" {
		endpointAssignment = fmt.Sprintf("endpoint = \"%s\"", endpointAssignment)
	}
	enabledAssignment := ""
	if !enabled {
		enabledAssignment = "enabled = false"
	}
	nonceAssignment := ""
	if nonce != nil {
		nonceAssignment = fmt.Sprintf("nonce = %d", *nonce)
	}
	sb := strings.Builder{}
	for k, v := range tags {
		sb.WriteString(fmt.Sprintf("%s = \"%s\"", k, v))
		sb.WriteString("\n")
	}
	r := fmt.Sprintf(`
provider "modtm" {
  %s
  %s
}

resource "modtm_telemetry" "test" {
  tags = {
   %s
  }
  %s
}
`, endpointAssignment, enabledAssignment, sb.String(), nonceAssignment)
	return r
}

func intPtr(i int) *int {
	return &i
}
