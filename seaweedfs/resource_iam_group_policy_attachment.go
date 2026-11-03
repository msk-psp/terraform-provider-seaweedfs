package seaweedfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &iamGroupPolicyAttachmentResource{}

func NewIAMGroupPolicyAttachmentResource() resource.Resource {
	return &iamGroupPolicyAttachmentResource{}
}

type iamGroupPolicyAttachmentResource struct{ data *providerData }
type iamGroupPolicyAttachmentModel struct {
	ID        types.String `tfsdk:"id"`
	GroupName types.String `tfsdk:"group_name"`
	PolicyARN types.String `tfsdk:"policy_arn"`
}

func (r *iamGroupPolicyAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_group_policy_attachment"
}
func (r *iamGroupPolicyAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages one managed-policy attachment to an IAM group. Import existing attachments before managing them.", Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true},
		"group_name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"policy_arn": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
	}}
}
func (r *iamGroupPolicyAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *providerData, got %T", req.ProviderData))
		return
	}
	r.data = data
}
func (r *iamGroupPolicyAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan iamGroupPolicyAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	group, arn := plan.GroupName.ValueString(), plan.PolicyARN.ValueString()
	err := r.data.withUserLock(group, func() error {
		exists, err := r.data.client.groupHasPolicy(ctx, group, arn)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("attachment already exists; import %s|%s before managing it", group, arn)
		}
		return r.data.client.attachGroupPolicy(ctx, group, arn)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to attach IAM group policy", err.Error())
		return
	}
	plan.ID = types.StringValue(group + "|" + arn)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *iamGroupPolicyAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state iamGroupPolicyAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.data.client.groupHasPolicy(ctx, state.GroupName.ValueString(), state.PolicyARN.ValueString())
	if isNoSuchEntityError(err) || (err == nil && !found) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read IAM group policy attachment", err.Error())
		return
	}
	state.ID = types.StringValue(state.GroupName.ValueString() + "|" + state.PolicyARN.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *iamGroupPolicyAttachmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Attachment changes require replacement.")
}
func (r *iamGroupPolicyAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state iamGroupPolicyAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.data.withUserLock(state.GroupName.ValueString(), func() error {
		return r.data.client.detachGroupPolicy(ctx, state.GroupName.ValueString(), state.PolicyARN.ValueString())
	})
	if err != nil && !isNoSuchEntityError(err) {
		resp.Diagnostics.AddError("Failed to detach IAM group policy", err.Error())
	}
}
func (r *iamGroupPolicyAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	group, arn, ok := strings.Cut(req.ID, "|")
	if !ok || group == "" || arn == "" || strings.Contains(arn, "|") {
		resp.Diagnostics.AddError("Invalid import ID", "Expected group_name|policy_arn.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_name"), group)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("policy_arn"), arn)...)
}
