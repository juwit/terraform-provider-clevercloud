package clevercloud

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/clevercloud/clevercloud-go/clevercloud"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type resourceApplicationType struct{}

func (r resourceApplicationType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"name": {
				Type:     types.StringType,
				Required: true,
			},
			"description": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"type": {
				Type:     types.StringType,
				Required: true,
			},
			"zone": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"deploy_type": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"deploy_url": {
				Type:     types.StringType,
				Computed: true,
			},
			"organization_id": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"scalability": {
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"min_instances": {
						Type:     types.NumberType,
						Optional: true,
						Computed: true,
					},
					"max_instances": {
						Type:     types.NumberType,
						Optional: true,
						Computed: true,
					},
					"min_flavor": {
						Type:     types.StringType,
						Optional: true,
						Computed: true,
					},
					"max_flavor": {
						Type:     types.StringType,
						Optional: true,
						Computed: true,
					},
					"max_allowed_instances": {
						Type:     types.NumberType,
						Computed: true,
					},
				}),
				Optional: true,
			},
			"properties": {
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"homogeneous": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
					"sticky_sessions": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
					"cancel_on_push": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
					"force_https": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
				}),
				Optional: true,
			},
			"build": {
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"separate_build": {
						Type:     types.BoolType,
						Optional: true,
						Computed: true,
					},
					"build_flavor": {
						Type:     types.StringType,
						Optional: true,
						Computed: true,
					},
				}),
				Optional: true,
			},
			// "environment": {
			// 	Type: types.MapType{
			// 		ElemType: types.StringType,
			// 	},
			// 	Optional: true,
			// },
			// "exposed_environment": {
			// 	Type: types.MapType{
			// 		ElemType: types.StringType,
			// 	},
			// 	Optional: true,
			// },
			// "dependencies": {
			// 	Type: types.MapType{
			// 		ElemType: types.StringType,
			// 	},
			// 	Optional: true,
			// },
			// "vhosts": {
			// 	Type: types.ListType{
			// 		ElemType: types.StringType,
			// 	},
			// 	Optional: true,
			// },
			"favorite": {
				Type:     types.BoolType,
				Optional: true,
				Computed: true,
			},
			"archived": {
				Type:     types.BoolType,
				Optional: true,
				Computed: true,
			},
			"tags": {
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Optional: true,
				Computed: true,
			},
		},
	}, nil
}

func (r resourceApplicationType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resourceApplication{
		p: *(p.(*provider)),
	}, nil
}

type resourceApplication struct {
	p provider
}

func (app Application) attributeDescriptionOrDefault() string {
	if app.Description.Unknown || app.Description.Null {
		return app.Name.Value
	}

	return app.Description.Value
}

func (app Application) attributeZoneOrDefault() string {
	if app.Zone.Unknown || app.Zone.Null {
		return "par"
	}

	return app.Zone.Value
}

func (app Application) attributeDeployTypeOrDefault() string {
	if app.DeployType.Unknown || app.DeployType.Null {
		return "GIT"
	}

	return app.DeployType.Value
}

func getInstanceByType(cc *clevercloud.APIClient, instanceType string) (*clevercloud.AvailableInstanceView, error) {
	instances, _, err := cc.ProductsApi.GetAvailableInstances(context.Background(), &clevercloud.GetAvailableInstancesOpts{})
	if err != nil {
		return nil, err
	}

	enabledInstances := make([]clevercloud.AvailableInstanceView, 0)
	for _, instance := range instances {
		if instance.Enabled {
			enabledInstances = append(enabledInstances, instance)
		}
	}

	matchingInstances := make([]clevercloud.AvailableInstanceView, 0)
	for _, instance := range enabledInstances {
		if instance.Variant.Slug == instanceType {
			matchingInstances = append(matchingInstances, instance)
		}
	}

	instanceVersions := make([]string, 0)
	for _, instance := range matchingInstances {
		instanceVersions = append(instanceVersions, instance.Version)
	}

	sort.Strings(instanceVersions)

	latestInstanceVersion := instanceVersions[len(instanceVersions)-1]

	var latestInstance clevercloud.AvailableInstanceView
	for _, instance := range matchingInstances {
		if instance.Version == latestInstanceVersion {
			latestInstance = instance
		}
	}

	return &latestInstance, nil
}

func (app Application) attributeForceHttpsToString() string {
	if app.Properties.ForceHTTPS.Value {
		return "ENABLED"
	} else {
		return "DISABLED"
	}
}

func forceHttpsToBool(value string) bool {
	if value == "ENABLED" {
		return true
	} else {
		return false
	}
}

func (r resourceApplication) applicationModelToWannabeApplication(ctx context.Context, plan *Application) (*clevercloud.WannabeApplication, diag.Diagnostic) {
	planApplicationInstance, err := getInstanceByType(r.p.client, plan.Type.Value)
	if err != nil {
		return nil, diag.NewErrorDiagnostic("Error fetching instance type from plan", "An unexpected error was encountered while reading the plan: "+err.Error())
	}

	defaultFlavorName := planApplicationInstance.DefaultFlavor.Name

	var planMinInstances int32 = 1
	if !plan.Scalability.MinInstances.Unknown && !plan.Scalability.MinInstances.Null {
		value, _ := plan.Scalability.MinInstances.Value.Int64()
		planMinInstances = int32(value)
	}

	var planMaxInstances int32 = 1
	if !plan.Scalability.MaxInstances.Unknown && !plan.Scalability.MaxInstances.Null {
		value, _ := plan.Scalability.MaxInstances.Value.Int64()
		planMaxInstances = int32(value)
	}

	wannabeApplication := clevercloud.WannabeApplication{
		Name:            plan.Name.Value,
		Description:     plan.attributeDescriptionOrDefault(),
		Zone:            plan.attributeZoneOrDefault(),
		Deploy:          plan.attributeDeployTypeOrDefault(),
		InstanceType:    plan.Type.Value,
		InstanceVersion: planApplicationInstance.Version,
		InstanceVariant: planApplicationInstance.Variant.Id,
		MinInstances:    planMinInstances,
		MaxInstances:    planMaxInstances,
		MinFlavor:       defaultFlavorName,
		MaxFlavor:       defaultFlavorName,
	}

	if !plan.Build.SeparateBuild.Unknown {
		wannabeApplication.SeparateBuild = plan.Build.SeparateBuild.Value
		wannabeApplication.BuildFlavor = planApplicationInstance.BuildFlavor.Name
	}

	for _, flavor := range planApplicationInstance.Flavors {
		if !plan.Scalability.MinFlavor.Unknown && plan.Scalability.MinFlavor.Value == flavor.Name {
			wannabeApplication.MinFlavor = flavor.Name
		}
		if !plan.Scalability.MaxFlavor.Unknown && plan.Scalability.MaxFlavor.Value == flavor.Name {
			wannabeApplication.MaxFlavor = flavor.Name
		}
		if !plan.Build.BuildFlavor.Unknown && plan.Build.BuildFlavor.Value == flavor.Name {
			wannabeApplication.BuildFlavor = flavor.Name
		}
	}

	if !plan.Properties.Homogeneous.Unknown {
		wannabeApplication.Homogeneous = !plan.Properties.Homogeneous.Value
	}

	if !plan.Properties.StickySessions.Unknown {
		wannabeApplication.StickySessions = plan.Properties.StickySessions.Value
	}

	if !plan.Properties.CancelOnPush.Unknown {
		wannabeApplication.CancelOnPush = plan.Properties.CancelOnPush.Value
	}

	if !plan.Properties.ForceHTTPS.Unknown {
		wannabeApplication.ForceHttps = plan.attributeForceHttpsToString()
	}

	if !plan.Favorite.Unknown {
		wannabeApplication.Favourite = plan.Favorite.Value
	}

	if !plan.Archived.Unknown {
		wannabeApplication.Archived = plan.Archived.Value
	}

	if diags := plan.Tags.ElementsAs(ctx, &wannabeApplication.Tags, false); diags != nil {
		return nil, diag.NewErrorDiagnostic("Error interfacing tags from plan", "An unexpected error was encountered while reading the plan.")
	}

	return &wannabeApplication, nil
}

func applicationViewToApplicationModel(application clevercloud.ApplicationView) Application {
	return Application{
		ID:             types.String{Value: application.Id},
		Name:           types.String{Value: application.Name},
		Description:    types.String{Value: application.Description},
		Type:           types.String{Value: application.Instance.Type},
		Zone:           types.String{Value: application.Zone},
		DeployType:     types.String{Value: application.Deployment.Type},
		DeployUrl:      types.String{Value: application.Deployment.Url},
		OrganizationID: types.String{Value: application.OwnerId},
		Scalability: ApplicationScalability{
			MinInstances:        types.Number{Value: big.NewFloat(float64(application.Instance.MinInstances))},
			MaxInstances:        types.Number{Value: big.NewFloat(float64(application.Instance.MaxInstances))},
			MinFlavor:           types.String{Value: application.Instance.MinFlavor.Name},
			MaxFlavor:           types.String{Value: application.Instance.MaxFlavor.Name},
			MaxAllowedInstances: types.Number{Value: big.NewFloat(float64(application.Instance.MaxAllowedInstances))},
		},
		Properties: ApplicationProperties{
			Homogeneous:    types.Bool{Value: !application.Homogeneous},
			StickySessions: types.Bool{Value: application.StickySessions},
			CancelOnPush:   types.Bool{Value: application.CancelOnPush},
			ForceHTTPS:     types.Bool{Value: forceHttpsToBool(application.ForceHttps)},
		},
		Build: ApplicationBuild{
			SeparateBuild: types.Bool{Value: application.SeparateBuild},
			BuildFlavor:   types.String{Value: application.BuildFlavor.Name},
		},
		Favorite: types.Bool{Value: application.Favourite},
		Archived: types.Bool{Value: application.Archived},
		Tags:     types.List{ElemType: types.StringType},
	}
}

func formatRequestClientError(err error) string {
	return fmt.Sprintf(
		"An unexpected error was encountered while requesting the API: %s\n\n%s\n",
		err.Error(),
		string(err.(clevercloud.GenericOpenAPIError).Body()),
	)
}

func (r resourceApplication) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError("Provider not configured", "The provider hasn't been configured before apply.")
		return
	}

	var plan Application
	if diags := req.Plan.Get(ctx, &plan); diags != nil {
		resp.Diagnostics.AddError("Error reading plan", "An unexpected error was encountered while reading the plan.")
		return
	}

	wannabeApplication, err := r.applicationModelToWannabeApplication(ctx, &plan)
	if err != nil {
		resp.Diagnostics = append(resp.Diagnostics, err)
		return
	}

	var application clevercloud.ApplicationView
	var tags []string

	if plan.OrganizationID.Unknown || plan.OrganizationID.Null {
		var err error
		_, _, err = r.p.client.SelfApi.GetUser(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Request error while fetching self user", formatRequestClientError(err))
			return
		}

		application, _, err = r.p.client.SelfApi.AddSelfApplication(ctx, *wannabeApplication)
		if err != nil {
			resp.Diagnostics.AddError("Request error while creating application for self", formatRequestClientError(err))
			return
		}

		tags, _, err = r.p.client.SelfApi.GetSelfApplicationTagsByAppId(ctx, application.Id)
		if err != nil {
			resp.Diagnostics.AddError("Request error while fetching application tags for self", formatRequestClientError(err))
			return
		}
	} else {
		var err error
		application, _, err = r.p.client.OrganisationApi.AddApplicationByOrga(ctx, plan.OrganizationID.Value, *wannabeApplication)
		if err != nil {
			resp.Diagnostics.AddError("Request error while creating application for organization: "+plan.OrganizationID.Value, formatRequestClientError(err))
			return
		}

		tags, _, err = r.p.client.OrganisationApi.GetApplicationTagsByOrgaAndAppId(ctx, plan.OrganizationID.Value, application.Id)
		if err != nil {
			resp.Diagnostics.AddError("Request error while fetching application tags for organization", formatRequestClientError(err))
			return
		}
	}

	var result = applicationViewToApplicationModel(application)

	for _, tag := range tags {
		result.Tags.Elems = append(result.Tags.Elems, types.String{Value: tag})
	}

	if diags := resp.State.Set(ctx, result); diags != nil {
		resp.Diagnostics.AddError("Error setting application state", "Could not set state, unexpected error.")
		return
	}
}

func (r resourceApplication) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state Application
	if diags := req.State.Get(ctx, &state); diags != nil {
		resp.Diagnostics.AddError("Error reading state", "An unexpected error was encountered while reading the state.")
		return
	}

	applicationID := state.ID.Value

	var application clevercloud.ApplicationView
	var tags []string

	if !state.OrganizationID.Null {
		var err error
		_, _, err = r.p.client.SelfApi.GetUser(context.Background())
		if err != nil {
			resp.Diagnostics.AddError("Request error while fetching self user", "An unexpected error was encountered while requesting the api: "+err.Error())
			return
		}

		application, _, err = r.p.client.SelfApi.GetSelfApplicationByAppId(context.Background(), applicationID)
		if err != nil {
			resp.Diagnostics.AddError("Request error while reading application for self", "An unexpected error was encountered while requesting the api: "+err.Error())
			return
		}

		tags, _, err = r.p.client.SelfApi.GetSelfApplicationTagsByAppId(context.Background(), application.Id)
		if err != nil {
			resp.Diagnostics.AddError("Request error while fetching application tags for self", "An unexpected error was encountered while requesting the api: "+err.Error())
			return
		}
	} else {
		var err error
		application, _, err = r.p.client.OrganisationApi.GetApplicationByOrgaAndAppId(context.Background(), state.OrganizationID.Value, applicationID)
		if err != nil {
			resp.Diagnostics.AddError("Request error while reading application for organization: "+state.OrganizationID.Value, "An unexpected error was encountered while requesting the api: "+err.Error())
			return
		}

		tags, _, err = r.p.client.OrganisationApi.GetApplicationTagsByOrgaAndAppId(context.Background(), state.OrganizationID.Value, application.Id)
		if err != nil {
			resp.Diagnostics.AddError("Request error while fetching application tags for organization", "An unexpected error was encountered while requesting the api: "+err.Error())
			return
		}
	}

	var result = applicationViewToApplicationModel(application)

	for _, tag := range tags {
		result.Tags.Elems = append(result.Tags.Elems, types.String{Value: tag})
	}

	if diags := resp.State.Set(ctx, result); diags != nil {
		resp.Diagnostics.AddError("Error setting state", "Unexpected error encountered trying to set new state.")
		return
	}
}

func (r resourceApplication) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
}

func (r resourceApplication) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
}

func (r resourceApplication) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	tfsdk.ResourceImportStateNotImplemented(ctx, "Coming soon", resp)
	return
}
