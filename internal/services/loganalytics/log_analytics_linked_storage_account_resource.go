package loganalytics

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/go-azure-helpers/lang/response"
	"github.com/hashicorp/go-azure-sdk/resource-manager/operationalinsights/2020-08-01/linkedstorageaccounts"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func resourceLogAnalyticsLinkedStorageAccount() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceLogAnalyticsLinkedStorageAccountCreateUpdate,
		Read:   resourceLogAnalyticsLinkedStorageAccountRead,
		Update: resourceLogAnalyticsLinkedStorageAccountCreateUpdate,
		Delete: resourceLogAnalyticsLinkedStorageAccountDelete,

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := linkedstorageaccounts.ParseDataSourceTypeIDInsensitively(id) // TODO remove insensitive parsing in 4.0
			return err
		}),

		Schema: map[string]*pluginsdk.Schema{
			"data_source_type": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					strings.ToLower(string(linkedstorageaccounts.DataSourceTypeCustomLogs)),
					strings.ToLower(string(linkedstorageaccounts.DataSourceTypeAzureWatson)),
					strings.ToLower(string(linkedstorageaccounts.DataSourceTypeQuery)),
					strings.ToLower(string(linkedstorageaccounts.DataSourceTypeAlerts)),
					// Value removed from enum in 2020-08-01, but effectively still works
					"ingestion",
				}, false),
			},

			"resource_group_name": azure.SchemaResourceGroupName(),

			"workspace_resource_id": {
				Type:         pluginsdk.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: linkedstorageaccounts.ValidateWorkspaceID,
			},

			"storage_account_ids": {
				Type:     pluginsdk.TypeSet,
				Required: true,
				MinItems: 1,
				Elem: &pluginsdk.Schema{
					Type:         pluginsdk.TypeString,
					ValidateFunc: azure.ValidateResourceID,
				},
			},
		},
	}
}

func resourceLogAnalyticsLinkedStorageAccountCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).LogAnalytics.LinkedStorageAccountClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	workspace, err := linkedstorageaccounts.ParseWorkspaceID(d.Get("workspace_resource_id").(string))
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	id := linkedstorageaccounts.NewDataSourceTypeID(workspace.SubscriptionId, d.Get("resource_group_name").(string), workspace.WorkspaceName, linkedstorageaccounts.DataSourceType(d.Get("data_source_type").(string)))

	if d.IsNewResource() {
		existing, err := client.Get(ctx, id)
		if err != nil {
			if !response.WasNotFound(existing.HttpResponse) {
				return fmt.Errorf("checking for presence of existing %s: %+v", id, err)
			}
		}
		if !response.WasNotFound(existing.HttpResponse) {
			return tf.ImportAsExistsError("azurerm_log_analytics_linked_storage_account", id.ID())
		}
	}

	parameters := linkedstorageaccounts.LinkedStorageAccountsResource{
		Properties: linkedstorageaccounts.LinkedStorageAccountsProperties{
			StorageAccountIds: utils.ExpandStringSlice(d.Get("storage_account_ids").(*pluginsdk.Set).List()),
		},
	}
	if _, err := client.CreateOrUpdate(ctx, id, parameters); err != nil {
		return fmt.Errorf("creating/updating %s: %+v", id, err)
	}

	d.SetId(id.ID())
	return resourceLogAnalyticsLinkedStorageAccountRead(d, meta)
}

func resourceLogAnalyticsLinkedStorageAccountRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).LogAnalytics.LinkedStorageAccountClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := linkedstorageaccounts.ParseDataSourceTypeIDInsensitively(d.Id()) // TODO remove insensitive parsing in 4.0
	if err != nil {
		return err
	}

	resp, err := client.Get(ctx, *id)
	if err != nil {
		if response.WasNotFound(resp.HttpResponse) {
			log.Printf("[INFO] Log Analytics Linked Storage Account %q does not exist - removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("retrieving %s: %+v", id, err)
	}

	d.Set("resource_group_name", id.ResourceGroupName)
	d.Set("workspace_resource_id", linkedstorageaccounts.NewWorkspaceID(id.SubscriptionId, id.ResourceGroupName, id.WorkspaceName).ID())

	if model := resp.Model; model != nil {
		props := model.Properties
		var storageAccountIds []string
		if props.StorageAccountIds != nil {
			storageAccountIds = *props.StorageAccountIds
		}
		d.Set("storage_account_ids", storageAccountIds)

		dataSourceType := ""
		if props.DataSourceType != nil {
			dataSourceType = string(*props.DataSourceType)
		}
		d.Set("data_source_type", dataSourceType)

	}

	return nil
}

func resourceLogAnalyticsLinkedStorageAccountDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).LogAnalytics.LinkedStorageAccountClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := linkedstorageaccounts.ParseDataSourceTypeIDInsensitively(d.Id()) // TODO remove insensitive parsing in 4.0
	if err != nil {
		return err
	}

	if _, err := client.Delete(ctx, *id); err != nil {
		return fmt.Errorf("deleting %s: %+v", id, err)
	}
	return nil
}
