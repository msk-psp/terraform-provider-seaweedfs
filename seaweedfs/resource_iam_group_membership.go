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

var _ resource.ResourceWithImportState = &iamGroupMembershipResource{}

func NewIAMGroupMembershipResource() resource.Resource { return &iamGroupMembershipResource{} }

type iamGroupMembershipResource struct{ data *providerData }
type iamGroupMembershipModel struct {
	ID        types.String `tfsdk:"id"`
	GroupName types.String `tfsdk:"group_name"`
	UserName  types.String `tfsdk:"user_name"`
}

func (r *iamGroupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_group_membership"
}
func (r *iamGroupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages one IAM group membership. Import existing memberships before managing them.", Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true},
		"group_name": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"user_name":  schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
	}}
}
func (r *iamGroupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *iamGroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan iamGroupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	group, user := plan.GroupName.ValueString(), plan.UserName.ValueString()
	err := r.data.withUserLock(group, func() error {
		exists, err := r.data.client.groupUser(ctx, group, user)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("membership already exists; import %s/%s before managing it", group, user)
		}
		return r.data.client.addGroupUser(ctx, group, user)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create IAM group membership", err.Error())
		return
	}
	plan.ID = types.StringValue(group + "/" + user)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *iamGroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state iamGroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.data.client.groupUser(ctx, state.GroupName.ValueString(), state.UserName.ValueString())
	if isNoSuchEntityError(err) || (err == nil && !found) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read IAM group membership", err.Error())
		return
	}
	state.ID = types.StringValue(state.GroupName.ValueString() + "/" + state.UserName.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *iamGroupMembershipResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Membership changes require replacement.")
}
func (r *iamGroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state iamGroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.data.withUserLock(state.GroupName.ValueString(), func() error {
		return r.data.client.removeGroupUser(ctx, state.GroupName.ValueString(), state.UserName.ValueString())
	})
	if err != nil && !isNoSuchEntityError(err) {
		resp.Diagnostics.AddError("Failed to delete IAM group membership", err.Error())
	}
}
func (r *iamGroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	group, user, ok := strings.Cut(req.ID, "/")
	if !ok || group == "" || user == "" || strings.Contains(user, "/") {
		resp.Diagnostics.AddError("Invalid import ID", "Expected group_name/user_name.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_name"), group)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_name"), user)...)
}
