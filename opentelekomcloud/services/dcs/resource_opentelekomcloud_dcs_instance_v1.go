package dcs

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/dcs/v1/instances"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceDcsInstanceV1() *schema.Resource {
	return &schema.Resource{
		Create: resourceDcsInstancesV1Create,
		Read:   resourceDcsInstancesV1Read,
		Update: resourceDcsInstancesV1Update,
		Delete: resourceDcsInstancesV1Delete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"engine": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"engine_version": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"capacity": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"password": {
				Type:      schema.TypeString,
				Sensitive: true,
				Required:  true,
				ForceNew:  true,
			},
			"access_user": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"security_group_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"available_zones": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"product_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"maintain_begin": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"maintain_end": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"save_days": {
				Type:       schema.TypeInt,
				Optional:   true,
				ForceNew:   true,
				Deprecated: "Please use `backup_policy` instead",
			},
			"backup_type": {
				Type:       schema.TypeString,
				Optional:   true,
				ForceNew:   true,
				Deprecated: "Please use `backup_policy` instead",
			},
			"begin_at": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				RequiredWith: []string{"period_type", "backup_at", "save_days", "backup_type"},
				Deprecated:   "Please use `backup_policy` instead",
			},
			"period_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				RequiredWith: []string{"begin_at", "backup_at", "save_days", "backup_type"},
				Deprecated:   "Please use `backup_policy` instead",
			},
			"backup_at": {
				Type:         schema.TypeList,
				Optional:     true,
				ForceNew:     true,
				RequiredWith: []string{"period_type", "begin_at", "save_days", "backup_type"},
				Deprecated:   "Please use `backup_policy` instead",
				Elem:         &schema.Schema{Type: schema.TypeInt},
			},
			"backup_policy": {
				Type:          schema.TypeList,
				Optional:      true,
				ConflictsWith: []string{"backup_type", "begin_at", "period_type", "backup_at", "save_days"},
				MaxItems:      1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"save_days": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"backup_type": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"begin_at": {
							Type:     schema.TypeString,
							Required: true,
						},
						"period_type": {
							Type:     schema.TypeString,
							Required: true,
						},
						"backup_at": {
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeInt},
						},
					},
				},
			},
			"order_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"vpc_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"security_group_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"subnet_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"port": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"resource_spec_code": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"used_memory": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"internal_version": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"max_memory": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"user_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"user_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ip": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func formatAts(src []interface{}) []int {
	res := make([]int, len(src))
	for i, at := range src {
		res[i] = at.(int)
	}
	return res
}

func getInstanceBackupPolicy(d *schema.ResourceData) *instances.InstanceBackupPolicy {
	var instanceBackupPolicy *instances.InstanceBackupPolicy
	if _, ok := d.GetOk("backup_policy"); !ok { // deprecated branch
		backupAts := d.Get("backup_at").([]interface{})
		instanceBackupPolicy = &instances.InstanceBackupPolicy{
			SaveDays:   d.Get("save_days").(int),
			BackupType: d.Get("backup_type").(string),
			PeriodicalBackupPlan: instances.PeriodicalBackupPlan{
				BeginAt:    d.Get("begin_at").(string),
				PeriodType: d.Get("period_type").(string),
				BackupAt:   formatAts(backupAts),
			},
		}
	}

	backupPolicyList := d.Get("backup_policy").([]interface{})
	if len(backupPolicyList) == 0 {
		return nil
	}
	backupPolicy := backupPolicyList[0].(map[string]interface{})
	backupAts := backupPolicy["backup_at"].([]interface{})
	instanceBackupPolicy = &instances.InstanceBackupPolicy{
		SaveDays:   backupPolicy["save_days"].(int),
		BackupType: backupPolicy["backup_type"].(string),
		PeriodicalBackupPlan: instances.PeriodicalBackupPlan{
			BeginAt:    backupPolicy["begin_at"].(string),
			PeriodType: backupPolicy["period_type"].(string),
			BackupAt:   formatAts(backupAts),
		},
	}

	return instanceBackupPolicy
}

func resourceDcsInstancesV1Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	DcsV1Client, err := config.DcsV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating dcs instance client: %s", err)
	}

	noPasswordAccess := "true"
	if d.Get("access_user").(string) != "" || d.Get("password").(string) != "" {
		noPasswordAccess = "false"
	}
	createOpts := &instances.CreateOps{
		Name:                 d.Get("name").(string),
		Description:          d.Get("description").(string),
		Engine:               d.Get("engine").(string),
		EngineVersion:        d.Get("engine_version").(string),
		Capacity:             d.Get("capacity").(int),
		NoPasswordAccess:     noPasswordAccess,
		Password:             d.Get("password").(string),
		AccessUser:           d.Get("access_user").(string),
		VPCID:                d.Get("vpc_id").(string),
		SecurityGroupID:      d.Get("security_group_id").(string),
		SubnetID:             d.Get("subnet_id").(string),
		AvailableZones:       common.GetAllAvailableZones(d),
		ProductID:            d.Get("product_id").(string),
		InstanceBackupPolicy: getInstanceBackupPolicy(d),
		MaintainBegin:        d.Get("maintain_begin").(string),
		MaintainEnd:          d.Get("maintain_end").(string),
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	v, err := instances.Create(DcsV1Client, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error creating instance: %s", err)
	}
	log.Printf("[INFO] instance ID: %s", v.InstanceID)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"CREATING"},
		Target:     []string{"RUNNING"},
		Refresh:    DcsInstancesV1StateRefreshFunc(DcsV1Client, v.InstanceID),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}
	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf(
			"Error waiting for instance (%s) to become ready: %s",
			v.InstanceID, err)
	}

	// Store the instance ID now
	d.SetId(v.InstanceID)

	return resourceDcsInstancesV1Read(d, meta)
}

func resourceDcsInstancesV1Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)

	DcsV1Client, err := config.DcsV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating dcs instance client: %s", err)
	}
	v, err := instances.Get(DcsV1Client, d.Id()).Extract()
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Dcs instance %s: %+v", d.Id(), v)

	d.SetId(v.InstanceID)
	mErr := multierror.Append(nil,
		d.Set("name", v.Name),
		d.Set("engine", v.Engine),
		d.Set("capacity", v.Capacity),
		d.Set("used_memory", v.UsedMemory),
		d.Set("max_memory", v.MaxMemory),
		d.Set("port", v.Port),
		d.Set("status", v.Status),
		d.Set("description", v.Description),
		d.Set("resource_spec_code", v.ResourceSpecCode),
		d.Set("internal_version", v.InternalVersion),
		d.Set("vpc_id", v.VPCID),
		d.Set("vpc_name", v.VPCName),
		d.Set("created_at", v.CreatedAt),
		d.Set("product_id", v.ProductID),
		d.Set("security_group_id", v.SecurityGroupID),
		d.Set("security_group_name", v.SecurityGroupName),
		d.Set("subnet_id", v.SubnetID),
		d.Set("subnet_name", v.SubnetName),
		d.Set("user_id", v.UserID),
		d.Set("user_name", v.UserName),
		d.Set("order_id", v.OrderID),
		d.Set("maintain_begin", v.MaintainBegin),
		d.Set("maintain_end", v.MaintainEnd),
		d.Set("access_user", v.AccessUser),
		d.Set("ip", v.IP),
	)
	return mErr.ErrorOrNil()
}

func resourceDcsInstancesV1Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	DcsV1Client, err := config.DcsV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error updating dcs instance client: %s", err)
	}
	var updateOpts instances.UpdateOpts
	if d.HasChange("name") {
		updateOpts.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		description := d.Get("description").(string)
		updateOpts.Description = &description
	}
	if d.HasChange("maintain_begin") {
		updateOpts.MaintainBegin = d.Get("maintain_begin").(string)
	}
	if d.HasChange("maintain_end") {
		updateOpts.MaintainEnd = d.Get("maintain_end").(string)
	}
	if d.HasChange("security_group_id") {
		updateOpts.SecurityGroupID = d.Get("security_group_id").(string)
	}
	if d.HasChange("backup_policy") {
		updateOpts.InstanceBackupPolicy = getInstanceBackupPolicy(d)
	}

	err = instances.Update(DcsV1Client, d.Id(), updateOpts).Err
	if err != nil {
		return fmt.Errorf("Error updating Dcs Instance: %s", err)
	}

	return resourceDcsInstancesV1Read(d, meta)
}

func resourceDcsInstancesV1Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	DcsV1Client, err := config.DcsV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating dcs instance client: %s", err)
	}

	_, err = instances.Get(DcsV1Client, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "instance")
	}

	err = instances.Delete(DcsV1Client, d.Id()).ExtractErr()
	if err != nil {
		return fmt.Errorf("Error deleting instance: %s", err)
	}

	// Wait for the instance to delete before moving on.
	log.Printf("[DEBUG] Waiting for instance (%s) to delete", d.Id())

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"DELETING", "RUNNING"},
		Target:     []string{"DELETED"},
		Refresh:    DcsInstancesV1StateRefreshFunc(DcsV1Client, d.Id()),
		Timeout:    d.Timeout(schema.TimeoutDelete),
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf(
			"Error waiting for instance (%s) to delete: %s",
			d.Id(), err)
	}

	log.Printf("[DEBUG] Dcs instance %s deactivated.", d.Id())
	d.SetId("")
	return nil
}

func DcsInstancesV1StateRefreshFunc(client *golangsdk.ServiceClient, instanceID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		v, err := instances.Get(client, instanceID).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				return v, "DELETED", nil
			}
			return nil, "", err
		}

		return v, v.Status, nil
	}
}
