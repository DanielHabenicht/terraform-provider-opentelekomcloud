package sfs

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/sfs_turbo/v1/shares"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceSFSTurboShareV1() *schema.Resource {
	return &schema.Resource{
		Create: resourceSFSTurboShareV1Create,
		Read:   resourceSFSTurboShareV1Read,
		Update: resourceSFSTurboShareV1Update,
		Delete: resourceSFSTurboShareV1Delete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: common.ValidateName,
			},
			"size": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(500),
			},
			"share_proto": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "NFS",
				ValidateFunc: validation.StringInSlice(
					[]string{"NFS"}, false,
				),
			},
			"share_type": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "STANDARD",
			},
			"availability_zone": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"security_group_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"crypt_key_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"version": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"export_location": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"available_capacity": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceSFSTurboShareV1Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.SfsTurboV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud SFSTurboV1 client: %s", err)
	}

	createOpts := shares.CreateOpts{
		Name:             d.Get("name").(string),
		Size:             d.Get("size").(int),
		ShareProto:       d.Get("share_proto").(string),
		ShareType:        d.Get("share_type").(string),
		VpcID:            d.Get("vpc_id").(string),
		SubnetID:         d.Get("subnet_id").(string),
		SecurityGroupID:  d.Get("security_group_id").(string),
		AvailabilityZone: d.Get("availability_zone").(string),
		Metadata: shares.Metadata{
			CryptKeyID: d.Get("crypt_key_id").(string),
		},
	}

	log.Printf("[DEBUG] Create SFS turbo with option: %+v", createOpts)
	share, err := shares.Create(client, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud SFS Turbo: %s", err)
	}

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"100"},
		Target:     []string{"200"},
		Refresh:    waitForSFSTurboStatus(client, share.ID),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      20 * time.Second,
		MinTimeout: 3 * time.Second,
	}
	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("error waiting for SFS Turbo (%s) to become ready: %s", share.ID, err)
	}

	d.SetId(share.ID)

	return resourceSFSTurboShareV1Read(d, meta)
}

func resourceSFSTurboShareV1Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.SfsTurboV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud SFSTurboV1 client: %s", err)
	}

	share, err := shares.Get(client, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "Error deleting SFS Turbo")
	}

	mErr := multierror.Append(nil,
		d.Set("name", share.Name),
		d.Set("share_proto", share.ShareProto),
		d.Set("share_type", share.ShareType),
		d.Set("vpc_id", share.VpcID),
		d.Set("subnet_id", share.SubnetID),
		d.Set("security_group_id", share.SecurityGroupID),
		d.Set("version", share.Version),
		d.Set("region", config.GetRegion(d)),
		d.Set("availability_zone", share.AvailabilityZone),
		d.Set("available_capacity", share.AvailCapacity),
		d.Set("export_location", share.ExportLocation),
		d.Set("crypt_key_id", share.CryptKeyID),
	)

	if mErr.ErrorOrNil() != nil {
		return mErr
	}

	// n.Size is a string of float64, should convert it to int
	if fSize, err := strconv.ParseFloat(share.Size, 64); err == nil {
		if err = d.Set("size", int(fSize)); err != nil {
			return fmt.Errorf("error reading size of SFS Turbo: %s", err)
		}
	}

	return nil
}

func resourceSFSTurboShareV1Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.SfsTurboV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud SFSTurboV1 client: %s", err)
	}

	if d.HasChange("size") {
		oldSize, newSize := d.GetChange("size")
		if oldSize.(int) > newSize.(int) {
			return fmt.Errorf("shrinking OpenTelekomCloud SFS Turbo size is not supported")
		}

		expandOpts := shares.ExpandOpts{
			Extend: shares.ExtendOpts{NewSize: newSize.(int)},
		}

		if err := shares.Expand(client, d.Id(), expandOpts).ExtractErr(); err != nil {
			return fmt.Errorf("error expanding OpenTelekomCloud Share File size: %s", err)
		}

		stateConf := &resource.StateChangeConf{
			Pending:    []string{"121"},
			Target:     []string{"221", "232"},
			Refresh:    waitForSFSTurboSubStatus(client, d.Id()),
			Timeout:    d.Timeout(schema.TimeoutDelete),
			Delay:      10 * time.Second,
			MinTimeout: 5 * time.Second,
		}

		_, err = stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("error updating OpenTelekomCloud SFS Turbo: %s", err)
		}
	}

	if d.HasChange("security_group_id") {
		securityGroupID := d.Get("security_group_id").(string)
		changeSGOpts := shares.ChangeSGOpts{
			ChangeSecurityGroup: shares.SecurityGroupOpts{
				SecurityGroupID: securityGroupID,
			},
		}

		if err := shares.ChangeSG(client, d.Id(), changeSGOpts).ExtractErr(); err != nil {
			return fmt.Errorf("error changing security group OpenTelekomCloud Share File size: %s", err)
		}

		stateConf := &resource.StateChangeConf{
			Pending:    []string{"121"},
			Target:     []string{"221", "232"},
			Refresh:    waitForSFSTurboSubStatus(client, d.Id()),
			Timeout:    d.Timeout(schema.TimeoutDelete),
			Delay:      10 * time.Second,
			MinTimeout: 5 * time.Second,
		}

		_, err = stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("error updating OpenTelekomCloud SFS Turbo: %s", err)
		}

	}

	return resourceSFSTurboShareV1Read(d, meta)
}

func resourceSFSTurboShareV1Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.SfsTurboV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud SFSTurboV1 client: %s", err)
	}

	if err := shares.Delete(client, d.Id()).ExtractErr(); err != nil {
		return common.CheckDeleted(d, err, "Error deleting SFS Turbo")
	}

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"100", "200"},
		Target:     []string{"deleted"},
		Refresh:    waitForSFSTurboStatus(client, d.Id()),
		Timeout:    d.Timeout(schema.TimeoutDelete),
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("error deleting OpenTelekomCloud SFS Turbo: %s", err)
	}

	d.SetId("")
	return nil
}

func waitForSFSTurboStatus(client *golangsdk.ServiceClient, shareID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		share, err := shares.Get(client, shareID).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[INFO] Successfully deleted OpenTelekomCloud Shared File System: %s", shareID)
				return share, "deleted", nil
			}
			return share, "error", err
		}
		if share.Status == "200" {
			return share, share.Status, nil
		}
		return share, share.Status, nil
	}
}

func waitForSFSTurboSubStatus(client *golangsdk.ServiceClient, shareID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		share, err := shares.Get(client, shareID).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[INFO] Successfully deleted OpenTelekomCloud Shared File System: %s", shareID)
				return share, "deleted", nil
			}
			return share, "error", err
		}
		if share.SubStatus == "221" || share.SubStatus == "232" {
			return share, share.SubStatus, nil
		}
		return share, share.SubStatus, nil
	}
}
