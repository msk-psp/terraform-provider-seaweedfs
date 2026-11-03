package seaweedfs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGroupPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch r.Form.Get("Action") {
		case "GetGroup":
			if r.Form.Get("Marker") == "" {
				fmt.Fprint(w, `<GetGroupResponse><GetGroupResult><Users><member><UserName>first</UserName></member></Users><IsTruncated>true</IsTruncated><Marker>next</Marker></GetGroupResult></GetGroupResponse>`)
			} else {
				fmt.Fprint(w, `<GetGroupResponse><GetGroupResult><Users><member><UserName>second</UserName></member></Users><IsTruncated>false</IsTruncated></GetGroupResult></GetGroupResponse>`)
			}
		case "ListAttachedGroupPolicies":
			fmt.Fprint(w, `<ListAttachedGroupPoliciesResponse><ListAttachedGroupPoliciesResult><IsTruncated>true</IsTruncated></ListAttachedGroupPoliciesResult></ListAttachedGroupPoliciesResponse>`)
		default:
			t.Fatalf("unexpected action %s", r.Form.Get("Action"))
		}
	}))
	defer srv.Close()
	client, err := newIAMClient(iamClientConfig{Endpoint: srv.URL, Region: "us-east-1", AccessKey: "test", SecretKey: "test"})
	if err != nil {
		t.Fatal(err)
	}
	found, err := client.groupUser(context.Background(), "group", "second")
	if err != nil || !found {
		t.Fatalf("second page membership: found=%v err=%v", found, err)
	}
	_, err = client.groupHasPolicy(context.Background(), "group", "arn:missing")
	if err == nil || !strings.Contains(err.Error(), "pagination marker") {
		t.Fatalf("expected invalid marker error, got %v", err)
	}
}

func TestManagedPolicyDocumentDecoding(t *testing.T) {
	for _, encoded := range []bool{false, true} {
		t.Run(fmt.Sprint(encoded), func(t *testing.T) {
			original := `{"Version":"2012-10-17","Statement":[],"Id":"a+b"}`
			document := original
			if encoded {
				document = url.QueryEscape(original)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				switch r.Form.Get("Action") {
				case "GetPolicy":
					fmt.Fprint(w, `<GetPolicyResponse><GetPolicyResult><Policy><PolicyName>test</PolicyName><DefaultVersionId>v1</DefaultVersionId></Policy></GetPolicyResult></GetPolicyResponse>`)
				case "GetPolicyVersion":
					fmt.Fprintf(w, `<GetPolicyVersionResponse><GetPolicyVersionResult><PolicyVersion><Document>%s</Document></PolicyVersion></GetPolicyVersionResult></GetPolicyVersionResponse>`, document)
				default:
					t.Fatal("unexpected action")
				}
			}))
			defer srv.Close()
			client, err := newIAMClient(iamClientConfig{Endpoint: srv.URL, Region: "us-east-1", AccessKey: "test", SecretKey: "test"})
			if err != nil {
				t.Fatal(err)
			}
			_, got, err := client.readManagedPolicy(context.Background(), "arn:test")
			if err != nil || got != original {
				t.Fatalf("got %q, err %v", got, err)
			}
		})
	}
}

func TestGroupDeletionRejectsExistingRelationships(t *testing.T) {
	for _, kind := range []string{"members", "attached", "inline", "empty"} {
		t.Run(kind, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				switch r.Form.Get("Action") {
				case "GetGroup":
					value := ""
					if kind == "members" {
						value = "<member><UserName>alice</UserName></member>"
					}
					fmt.Fprintf(w, "<GetGroupResponse><GetGroupResult><Users>%s</Users></GetGroupResult></GetGroupResponse>", value)
				case "ListAttachedGroupPolicies":
					value := ""
					if kind == "attached" {
						value = "<member><PolicyArn>arn:test</PolicyArn></member>"
					}
					fmt.Fprintf(w, "<ListAttachedGroupPoliciesResponse><ListAttachedGroupPoliciesResult><AttachedPolicies>%s</AttachedPolicies></ListAttachedGroupPoliciesResult></ListAttachedGroupPoliciesResponse>", value)
				case "ListGroupPolicies":
					value := ""
					if kind == "inline" {
						value = "<member>inline</member>"
					}
					fmt.Fprintf(w, "<ListGroupPoliciesResponse><ListGroupPoliciesResult><PolicyNames>%s</PolicyNames></ListGroupPoliciesResult></ListGroupPoliciesResponse>", value)
				default:
					t.Fatal("unexpected action")
				}
			}))
			defer srv.Close()
			client, err := newIAMClient(iamClientConfig{Endpoint: srv.URL, Region: "us-east-1", AccessKey: "test", SecretKey: "test"})
			if err != nil {
				t.Fatal(err)
			}
			err = client.ensureGroupEmpty(context.Background(), "group")
			if (err == nil) != (kind == "empty") {
				t.Fatalf("kind %s: unexpected error %v", kind, err)
			}
		})
	}
}
