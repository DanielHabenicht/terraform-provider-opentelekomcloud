package iam

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/identity/v3/projects"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceIdentityProjectV3() *schema.Resource {
	return &schema.Resource{
		Create: resourceIdentityProjectV3Create,
		Read:   resourceIdentityProjectV3Read,
		Update: resourceIdentityProjectV3Update,
		Delete: resourceIdentityProjectV3Delete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"domain_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
			},

			"parent_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"enabled": {
				Type:     schema.TypeBool,
				Computed: true,
			},

			"is_domain": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func resourceIdentityProjectV3Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	identityClient, err := config.IdentityV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack identity client: %s", err)
	}

	createOpts := projects.CreateOpts{
		Description: d.Get("description").(string),
		DomainID:    d.Get("domain_id").(string),
		Name:        d.Get("name").(string),
		ParentID:    d.Get("parent_id").(string),
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	project, err := projects.Create(identityClient, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error creating OpenStack project: %s", err)
	}

	d.SetId(project.ID)

	// some hacks here, due to GET API may return 404 after creation.
	for i := 0; i < 10; i++ {
		_, err := projects.Get(identityClient, d.Id()).Extract()
		if err != nil {
			time.Sleep(5 * time.Second)
		}
		break
	}

	return resourceIdentityProjectV3Read(d, meta)
}

func resourceIdentityProjectV3Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	identityClient, err := config.IdentityV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack identity client: %s", err)
	}

	project, err := projects.Get(identityClient, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "project")
	}

	log.Printf("[DEBUG] Retrieved OpenStack project: %#v", project)

	d.Set("description", project.Description)
	d.Set("domain_id", project.DomainID)
	d.Set("enabled", project.Enabled)
	d.Set("is_domain", project.IsDomain)
	d.Set("name", project.Name)
	d.Set("parent_id", project.ParentID)
	d.Set("region", config.GetRegion(d))

	return nil
}

func resourceIdentityProjectV3Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	identityClient, err := config.IdentityV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack identity client: %s", err)
	}

	var hasChange bool
	var updateOpts projects.UpdateOpts

	if d.HasChange("name") {
		hasChange = true
		updateOpts.Name = d.Get("name").(string)
	}

	if d.HasChange("description") {
		hasChange = true
		description := d.Get("description").(string)
		updateOpts.Description = description
	}

	if hasChange {
		_, err := projects.Update(identityClient, d.Id(), updateOpts).Extract()
		if err != nil {
			return fmt.Errorf("Error updating OpenStack project: %s", err)
		}
	}

	return resourceIdentityProjectV3Read(d, meta)
}

func resourceIdentityProjectV3Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	identityClient, err := config.IdentityV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack identity client: %s", err)
	}

	err = projects.Delete(identityClient, d.Id()).ExtractErr()
	if err != nil {
		return fmt.Errorf("Error deleting OpenStack project: %s", err)
	}

	return nil
}
