package seaweedfs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithImportState = &iamManagedPolicyResource{}

func NewIAMManagedPolicyResource() resource.Resource { return &iamManagedPolicyResource{} }

type iamManagedPolicyResource struct{ data *providerData }
type iamManagedPolicyModel struct {
	ID     types.String `tfsdk:"id"`
	ARN    types.String `tfsdk:"arn"`
	Name   types.String `tfsdk:"name"`
	Policy types.String `tfsdk:"policy"`
}

func (r *iamManagedPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_managed_policy"
}
func (r *iamManagedPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages a SeaweedFS IAM managed policy.", Attributes: map[string]schema.Attribute{
		"id":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"arn":    schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":   schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"policy": schema.StringAttribute{Required: true, Description: "JSON policy document."},
	}}
}
func (r *iamManagedPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *iamManagedPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan iamManagedPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !json.Valid([]byte(plan.Policy.ValueString())) {
		resp.Diagnostics.AddError("Invalid policy", "Policy must be valid JSON.")
		return
	}
	var created managedPolicy
	err := r.data.withUserLock(plan.Name.ValueString(), func() error {
		var err error
		created, err = r.data.client.createManagedPolicy(ctx, plan.Name.ValueString(), plan.Policy.ValueString())
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create IAM managed policy", err.Error())
		return
	}
	if created.ARN == "" {
		resp.Diagnostics.AddError("Missing policy ARN", "IAM created the policy but returned no ARN; inspect and import it before retrying.")
		return
	}
	plan.ID = types.StringValue(created.ARN)
	plan.ARN = types.StringValue(created.ARN)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *iamManagedPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state iamManagedPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	arn := state.ID.ValueString()
	info, doc, err := r.data.client.readManagedPolicy(ctx, arn)
	if isNoSuchEntityError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read IAM managed policy", err.Error())
		return
	}
	if info.Name == "" {
		resp.Diagnostics.AddError("Invalid policy metadata", "IAM returned a policy without a name.")
		return
	}
	state.ARN = types.StringValue(arn)
	state.Name = types.StringValue(info.Name)
	if !policiesSemanticallyEqual(state.Policy.ValueString(), doc) {
		state.Policy = types.StringValue(doc)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *iamManagedPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan iamManagedPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !json.Valid([]byte(plan.Policy.ValueString())) {
		resp.Diagnostics.AddError("Invalid policy", "Policy must be valid JSON.")
		return
	}
	err := r.data.withUserLock(plan.ID.ValueString(), func() error {
		return r.data.client.updateManagedPolicy(ctx, plan.ID.ValueString(), plan.Policy.ValueString())
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update IAM managed policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *iamManagedPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state iamManagedPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.data.withUserLock(state.ID.ValueString(), func() error { return r.data.client.deleteManagedPolicy(ctx, state.ID.ValueString()) })
	if err != nil && !isNoSuchEntityError(err) {
		resp.Diagnostics.AddError("Failed to delete IAM managed policy", err.Error())
	}
}
func (r *iamManagedPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
