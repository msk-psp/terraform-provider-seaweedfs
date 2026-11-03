package seaweedfs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

func (c *iamClient) groupAction(ctx context.Context, action string, args map[string]string, result any) error {
	values := url.Values{"Action": {action}, "Version": {"2010-05-08"}}
	for key, value := range args {
		values.Set(key, value)
	}
	return c.doIAMAction(ctx, values, result)
}

func (c *iamClient) createGroup(ctx context.Context, name string) error {
	return c.groupAction(ctx, "CreateGroup", map[string]string{"GroupName": name}, nil)
}

func (c *iamClient) deleteGroup(ctx context.Context, name string) error {
	return c.groupAction(ctx, "DeleteGroup", map[string]string{"GroupName": name}, nil)
}

// Refuse to remove access relationships not managed by this resource.
func (c *iamClient) ensureGroupEmpty(ctx context.Context, name string) error {
	var members groupResult
	if err := c.groupAction(ctx, "GetGroup", map[string]string{"GroupName": name}, &members); err != nil {
		return err
	}
	if len(members.Users) > 0 || members.Truncated {
		return fmt.Errorf("group %q still has members", name)
	}
	var attached struct {
		Items []struct {
			ARN string `xml:"PolicyArn"`
		} `xml:"ListAttachedGroupPoliciesResult>AttachedPolicies>member"`
		Truncated bool `xml:"ListAttachedGroupPoliciesResult>IsTruncated"`
	}
	if err := c.groupAction(ctx, "ListAttachedGroupPolicies", map[string]string{"GroupName": name}, &attached); err != nil {
		return err
	}
	if len(attached.Items) > 0 || attached.Truncated {
		return fmt.Errorf("group %q still has attached policies", name)
	}
	var inline struct {
		Items     []string `xml:"ListGroupPoliciesResult>PolicyNames>member"`
		Truncated bool     `xml:"ListGroupPoliciesResult>IsTruncated"`
	}
	if err := c.groupAction(ctx, "ListGroupPolicies", map[string]string{"GroupName": name}, &inline); err != nil {
		return err
	}
	if len(inline.Items) > 0 || inline.Truncated {
		return fmt.Errorf("group %q still has inline policies", name)
	}
	return nil
}

type groupResult struct {
	Group struct {
		Name string `xml:"GroupName"`
	} `xml:"GetGroupResult>Group"`
	Users []struct {
		Name string `xml:"UserName"`
	} `xml:"GetGroupResult>Users>member"`
	Truncated bool   `xml:"GetGroupResult>IsTruncated"`
	Marker    string `xml:"GetGroupResult>Marker"`
}

func (c *iamClient) groupUser(ctx context.Context, name, user string) (bool, error) {
	marker := ""
	for pages := 0; pages < 1000; pages++ {
		var out groupResult
		args := map[string]string{"GroupName": name}
		if marker != "" {
			args["Marker"] = marker
		}
		if err := c.groupAction(ctx, "GetGroup", args, &out); err != nil {
			return false, err
		}
		if user == "" {
			return true, nil
		}
		for _, member := range out.Users {
			if member.Name == user {
				return true, nil
			}
		}
		if !out.Truncated {
			return false, nil
		}
		if out.Marker == "" || out.Marker == marker {
			return false, fmt.Errorf("invalid GetGroup pagination marker")
		}
		marker = out.Marker
	}
	return false, fmt.Errorf("GetGroup pagination limit exceeded")
}

func (c *iamClient) addGroupUser(ctx context.Context, group, user string) error {
	return c.groupAction(ctx, "AddUserToGroup", map[string]string{"GroupName": group, "UserName": user}, nil)
}

func (c *iamClient) removeGroupUser(ctx context.Context, group, user string) error {
	return c.groupAction(ctx, "RemoveUserFromGroup", map[string]string{"GroupName": group, "UserName": user}, nil)
}

type managedPolicy struct {
	ARN     string `xml:"Arn"`
	Name    string `xml:"PolicyName"`
	Version string `xml:"DefaultVersionId"`
}

func (c *iamClient) createManagedPolicy(ctx context.Context, name, doc string) (managedPolicy, error) {
	var out struct {
		Policy managedPolicy `xml:"CreatePolicyResult>Policy"`
	}
	err := c.groupAction(ctx, "CreatePolicy", map[string]string{"PolicyName": name, "PolicyDocument": doc}, &out)
	return out.Policy, err
}

func (c *iamClient) readManagedPolicy(ctx context.Context, arn string) (managedPolicy, string, error) {
	var metadata struct {
		Policy managedPolicy `xml:"GetPolicyResult>Policy"`
	}
	if err := c.groupAction(ctx, "GetPolicy", map[string]string{"PolicyArn": arn}, &metadata); err != nil {
		return managedPolicy{}, "", err
	}
	if metadata.Policy.Version == "" {
		return managedPolicy{}, "", fmt.Errorf("policy has no default version")
	}
	var document struct {
		Value string `xml:"GetPolicyVersionResult>PolicyVersion>Document"`
	}
	if err := c.groupAction(ctx, "GetPolicyVersion", map[string]string{"PolicyArn": arn, "VersionId": metadata.Policy.Version}, &document); err != nil {
		// The policy exists; a missing version does not mean the policy itself is gone.
		return managedPolicy{}, "", fmt.Errorf("failed to read default policy version: %v", err)
	}
	decoded := document.Value
	if !json.Valid([]byte(decoded)) {
		var err error
		decoded, err = url.QueryUnescape(decoded)
		if err != nil {
			return managedPolicy{}, "", fmt.Errorf("invalid policy encoding: %w", err)
		}
	}
	if !json.Valid([]byte(decoded)) {
		return managedPolicy{}, "", fmt.Errorf("invalid policy document returned by IAM")
	}
	return metadata.Policy, decoded, nil
}

func (c *iamClient) updateManagedPolicy(ctx context.Context, arn, doc string) error {
	return c.groupAction(ctx, "CreatePolicyVersion", map[string]string{"PolicyArn": arn, "PolicyDocument": doc, "SetAsDefault": "true"}, nil)
}

func (c *iamClient) deleteManagedPolicy(ctx context.Context, arn string) error {
	return c.groupAction(ctx, "DeletePolicy", map[string]string{"PolicyArn": arn}, nil)
}

func (c *iamClient) attachGroupPolicy(ctx context.Context, group, arn string) error {
	return c.groupAction(ctx, "AttachGroupPolicy", map[string]string{"GroupName": group, "PolicyArn": arn}, nil)
}

func (c *iamClient) detachGroupPolicy(ctx context.Context, group, arn string) error {
	return c.groupAction(ctx, "DetachGroupPolicy", map[string]string{"GroupName": group, "PolicyArn": arn}, nil)
}

func (c *iamClient) groupHasPolicy(ctx context.Context, group, arn string) (bool, error) {
	marker := ""
	for pages := 0; pages < 1000; pages++ {
		var out struct {
			Policies []struct {
				ARN string `xml:"PolicyArn"`
			} `xml:"ListAttachedGroupPoliciesResult>AttachedPolicies>member"`
			Truncated bool   `xml:"ListAttachedGroupPoliciesResult>IsTruncated"`
			Marker    string `xml:"ListAttachedGroupPoliciesResult>Marker"`
		}
		args := map[string]string{"GroupName": group}
		if marker != "" {
			args["Marker"] = marker
		}
		if err := c.groupAction(ctx, "ListAttachedGroupPolicies", args, &out); err != nil {
			return false, err
		}
		for _, policy := range out.Policies {
			if policy.ARN == arn {
				return true, nil
			}
		}
		if !out.Truncated {
			return false, nil
		}
		if out.Marker == "" || out.Marker == marker {
			return false, fmt.Errorf("invalid ListAttachedGroupPolicies pagination marker")
		}
		marker = out.Marker
	}
	return false, fmt.Errorf("ListAttachedGroupPolicies pagination limit exceeded")
}
