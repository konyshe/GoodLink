package upnp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIsValid(t *testing.T) {
	var nilUpnp *Upnp
	if nilUpnp.IsValid() {
		t.Fatal("nil Upnp should be invalid")
	}
	if (&Upnp{}).IsValid() {
		t.Fatal("empty Upnp should be invalid")
	}
	u := &Upnp{
		LocalHost:        "192.168.1.2",
		GatewayOutsideIP: "1.2.3.4",
		CtrlUrl:          "/ctl",
		Gateway:          &Gateway{},
	}
	if u.IsValid() {
		t.Fatal("missing ServiceType should be invalid")
	}
	u.Gateway.ServiceType = "urn:schemas-upnp-org:service:WANIPConnection:1"
	if !u.IsValid() {
		t.Fatal("fully populated Upnp should be valid")
	}
}

func TestBuildXML(t *testing.T) {
	n := Node{
		Name: "SOAP-ENV:Envelope",
		Attr: map[string]string{
			"xmlns:SOAP-ENV":         `"http://schemas.xmlsoap.org/soap/envelope/"`,
			"SOAP-ENV:encodingStyle": `"http://schemas.xmlsoap.org/soap/encoding/"`,
		},
	}
	child := Node{Name: "SOAP-ENV:Body"}
	child.AddChild(Node{Name: "m:GetExternalIPAddress", Attr: map[string]string{"xmlns:m": `"urn:x"`}})
	n.AddChild(child)

	got := n.BuildXML()
	want := `<SOAP-ENV:Envelope SOAP-ENV:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/" xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/"><SOAP-ENV:Body><m:GetExternalIPAddress xmlns:m="urn:x"></m:GetExternalIPAddress></SOAP-ENV:Body></SOAP-ENV:Envelope>`
	if got != want {
		t.Fatalf("BuildXML mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildSOAPEnvelope(t *testing.T) {
	xml := buildSOAPEnvelope("urn:schemas-upnp-org:service:WANIPConnection:1", "AddPortMapping", []Node{
		{Name: "NewExternalPort", Content: "1234"},
		{Name: "NewRemoteHost"},
	})
	if !strings.Contains(xml, "<m:AddPortMapping xmlns:m=\"urn:schemas-upnp-org:service:WANIPConnection:1\">") {
		t.Fatalf("missing action node: %s", xml)
	}
	if !strings.Contains(xml, "<NewExternalPort>1234</NewExternalPort>") {
		t.Fatalf("missing arg: %s", xml)
	}
	if !strings.Contains(xml, "<NewRemoteHost></NewRemoteHost>") {
		t.Fatalf("missing empty arg: %s", xml)
	}
}

func TestResolveControlURL(t *testing.T) {
	base, err := url.Parse("http://192.168.1.1:1900/rootDesc.xml?n=1")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		in       string
		wantHost string
		wantPath string
	}{
		{"/upnp/control/WANIPConn1", "192.168.1.1:1900", "/upnp/control/WANIPConn1"},
		{"ctl/IPConn", "192.168.1.1:1900", "/ctl/IPConn"},
		{"http://192.168.1.1:5000/ctl", "192.168.1.1:5000", "/ctl"},
		{"/ctl?x=1", "192.168.1.1:1900", "/ctl?x=1"},
	}
	for _, tc := range cases {
		host, path := resolveControlURL(base, tc.in)
		if host != tc.wantHost || path != tc.wantPath {
			t.Errorf("resolveControlURL(%q) = %q %q, want %q %q", tc.in, host, path, tc.wantHost, tc.wantPath)
		}
	}
}

func TestParseGatewayFromSSDP(t *testing.T) {
	raw := "HTTP/1.1 200 OK\r\n" +
		"ST: urn:schemas-upnp-org:service:WANIPConnection:1\r\n" +
		"LOCATION: http://192.168.1.1:1900/rootDesc.xml?n=1\r\n" +
		"SERVER: TestGateway\r\n" +
		"USN: uuid:abc\r\n" +
		"CACHE-CONTROL: max-age=100\r\n\r\n"
	gw := parseGatewayFromSSDP(raw)
	if gw.Host != "192.168.1.1:1900" {
		t.Fatalf("Host = %q", gw.Host)
	}
	if gw.DeviceDescUrl != "/rootDesc.xml?n=1" {
		t.Fatalf("DeviceDescUrl = %q", gw.DeviceDescUrl)
	}
	if gw.ST != "urn:schemas-upnp-org:service:WANIPConnection:1" {
		t.Fatalf("ST = %q", gw.ST)
	}
	if gw.GatewayName != "TestGateway" {
		t.Fatalf("GatewayName = %q", gw.GatewayName)
	}
}

func TestParseControlURLFromDesc(t *testing.T) {
	desc := `
<root>
  <device>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
        <controlURL>/upnp/control/WANIPConn1</controlURL>
      </service>
    </serviceList>
  </device>
</root>`
	got := parseControlURLFromDesc(desc, "urn:schemas-upnp-org:service:WANIPConnection:1")
	if got != "/upnp/control/WANIPConn1" {
		t.Fatalf("controlURL = %q", got)
	}

	u := &Upnp{
		Gateway: &Gateway{
			Host:          "192.168.1.1:1900",
			DeviceDescUrl: "/rootDesc.xml?n=1",
			ServiceType:   "urn:schemas-upnp-org:service:WANIPConnection:1",
		},
	}
	d := DeviceDesc{upnp: u}
	d.resolve(desc)
	if u.CtrlUrl != "/upnp/control/WANIPConn1" {
		t.Fatalf("CtrlUrl = %q", u.CtrlUrl)
	}
	if u.Gateway.Host != "192.168.1.1:1900" {
		t.Fatalf("Host = %q", u.Gateway.Host)
	}
}

func TestParsePortMappingEntry(t *testing.T) {
	body := `<s:Envelope><s:Body>
		<NewExternalPort>12345</NewExternalPort>
		<NewProtocol>UDP</NewProtocol>
		<NewInternalPort>12345</NewInternalPort>
		<NewInternalClient>192.168.1.2</NewInternalClient>
		<NewPortMappingDescription>goodlink</NewPortMappingDescription>
		<NewEnabled>1</NewEnabled>
	</s:Body></s:Envelope>`
	entry := parsePortMappingEntry(body)
	if entry.ExternalPort != 12345 || entry.Protocol != "UDP" || entry.Description != "goodlink" || !entry.Enabled {
		t.Fatalf("%+v", entry)
	}
}

func TestParseExternalIP(t *testing.T) {
	body := `<s:Envelope><NewExternalIPAddress>8.8.4.4</NewExternalIPAddress></s:Envelope>`
	if got := parseExternalIP(body); got != "8.8.4.4" {
		t.Fatalf("ip = %q", got)
	}
	if got := parseExternalIP(`<s:Envelope></s:Envelope>`); got != "" {
		t.Fatalf("empty ip = %q", got)
	}
}

func TestSoapPost(t *testing.T) {
	var gotAction, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction = r.Header.Get("SOAPAction")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<NewExternalIPAddress>1.2.3.4</NewExternalIPAddress>`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	up := &Upnp{
		LocalHost: "192.168.1.2",
		CtrlUrl:   "/",
		Gateway: &Gateway{
			Host:        u.Host,
			ServiceType: "urn:schemas-upnp-org:service:WANIPConnection:1",
		},
		http_client: srv.Client(),
	}
	body, status, err := up.soapPost("GetExternalIPAddress", nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(string(body), "1.2.3.4") {
		t.Fatalf("body = %s", body)
	}
	if gotAction != `"urn:schemas-upnp-org:service:WANIPConnection:1#GetExternalIPAddress"` {
		t.Fatalf("SOAPAction = %s", gotAction)
	}
	if !strings.Contains(gotBody, "m:GetExternalIPAddress") {
		t.Fatalf("request body = %s", gotBody)
	}

	eia := ExternalIPAddress{upnp: up}
	if !eia.Send() || up.GatewayOutsideIP != "1.2.3.4" {
		t.Fatalf("ExternalIP Send failed: %q", up.GatewayOutsideIP)
	}
}

func TestAddKeepPortNilMap(t *testing.T) {
	u := &Upnp{}
	u.AddKeepPort(1234)
	if !u.KeepPorts[1234] {
		t.Fatal("KeepPorts not initialized")
	}
	u.RemoveKeepPort(1234)
	if u.KeepPorts[1234] {
		t.Fatal("RemoveKeepPort failed")
	}
}

func TestInitDoesNotResetKeepPorts(t *testing.T) {
	u := &Upnp{
		KeepPorts: map[int]bool{9999: true},
		LocalHost: "127.0.0.1",
		CtrlUrl:   "/ctl",
	}
	if err := u.Init(); err == nil {
		t.Fatal("expected Init to fail without gateway")
	}
	if !u.KeepPorts[9999] {
		t.Fatal("Init should not drop existing KeepPorts")
	}
}

func TestSearchGatewayTimeout(t *testing.T) {
	u := &Upnp{LocalHost: "127.0.0.1"}
	sg := SearchGateway{upnp: u}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	results := make(chan string, 1)
	start := time.Now()
	sg.send(ctx, "127.0.0.1:9", sg.buildRequest("WANIPConnection"), results)
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("send did not respect deadline, took %s", elapsed)
	}
	select {
	case got := <-results:
		t.Fatalf("unexpected result: %q", got)
	default:
	}
}

func TestSearchGatewayInvalidLocalNoFatal(t *testing.T) {
	u := &Upnp{LocalHost: "not-an-ip"}
	sg := SearchGateway{upnp: u}
	start := time.Now()
	if sg.Send() {
		t.Fatal("expected search failure")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("Send hung")
	}
}

func TestDeviceDescSendNilGateway(t *testing.T) {
	d := DeviceDesc{upnp: &Upnp{}}
	if d.Send() {
		t.Fatal("expected failure without gateway")
	}
}

func TestCleanMappingsIndexCap(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<NewExternalPort>1</NewExternalPort><NewProtocol>UDP</NewProtocol><NewPortMappingDescription>other</NewPortMappingDescription>`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	up := &Upnp{
		LocalHost:        "192.168.1.2",
		GatewayOutsideIP: "1.2.3.4",
		CtrlUrl:          "/",
		KeepPorts:        map[int]bool{},
		Gateway: &Gateway{
			Host:        u.Host,
			ServiceType: "urn:schemas-upnp-org:service:WANIPConnection:1",
		},
		http_client: srv.Client(),
	}
	up.CleanMappings()
	if calls != maxPortMappingIndex {
		t.Fatalf("calls = %d, want %d", calls, maxPortMappingIndex)
	}
}
