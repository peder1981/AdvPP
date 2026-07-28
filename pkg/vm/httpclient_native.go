package vm

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"golang.org/x/crypto/pkcs12"
)

func (v *VM) registerHttpNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	natives["FWHTTPGET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.fwHTTPRequest("GET", args)
	}
	natives["FWHTTPPOST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.fwHTTPRequest("POST", args)
	}
	natives["FWHTTPPUT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.fwHTTPRequest("PUT", args)
	}
	natives["FWHTTPPATCH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.fwHTTPRequest("PATCH", args)
	}
	natives["FWHTTPDELETE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.fwHTTPRequest("DELETE", args)
	}
	natives["FWHTTPBODY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(v.httpLastBody), nil
	}
	natives["FWHTTPSTATUS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(float64(v.httpLastStatus)), nil
	}
	natives["FWHTTPERROR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(v.httpLastError), nil
	}
}

func (v *VM) fwHTTPRequest(method string, args []advplrt.Value) (advplrt.Value, error) {
	v.httpLastBody = ""
	v.httpLastStatus = 0
	v.httpLastError = ""

	if len(args) < 1 {
		v.httpLastError = "URL is required"
		return advplrt.NewNumber(0), nil
	}

	url := advplrt.ToString(args[0])

	var body string
	var contentType string
	var certPath string
	var certPass string
	argIdx := 1

	if method == "POST" || method == "PUT" || method == "PATCH" {
		if len(args) > argIdx {
			body = advplrt.ToString(args[argIdx])
		}
		argIdx++
		if len(args) > argIdx {
			contentType = advplrt.ToString(args[argIdx])
		}
		argIdx++
	}
	if len(args) > argIdx {
		certPath = advplrt.ToString(args[argIdx])
	}
	argIdx++
	if len(args) > argIdx {
		certPass = advplrt.ToString(args[argIdx])
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	if certPath != "" && certPass != "" {
		pfxData, err := os.ReadFile(certPath)
		if err != nil {
			v.httpLastError = fmt.Sprintf("Failed to read cert: %v", err)
			return advplrt.NewNumber(0), nil
		}

		privateKey, cert, err := pkcs12.Decode(pfxData, certPass)
		if err != nil {
			v.httpLastError = fmt.Sprintf("Failed to decode PKCS12: %v", err)
			return advplrt.NewNumber(0), nil
		}

		tlsConfig.Certificates = []tls.Certificate{
			{
				Certificate: [][]byte{cert.Raw},
				PrivateKey:  privateKey,
			},
		}
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		v.httpLastError = fmt.Sprintf("Failed to create request: %v", err)
		return advplrt.NewNumber(0), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		v.httpLastError = fmt.Sprintf("Request failed: %v", err)
		return advplrt.NewNumber(0), nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		v.httpLastError = fmt.Sprintf("Failed to read response: %v", err)
		return advplrt.NewNumber(0), nil
	}

	v.httpLastStatus = resp.StatusCode
	v.httpLastBody = string(respBody)

	return advplrt.NewNumber(float64(resp.StatusCode)), nil
}
