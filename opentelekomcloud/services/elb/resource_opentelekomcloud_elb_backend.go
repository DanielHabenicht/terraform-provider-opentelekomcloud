package elb

import (
	"fmt"
	"log"
	"time"

	// "github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/elbaas/backendmember"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceBackend() *schema.Resource {
	return &schema.Resource{
		Create: resourceBackendCreate,
		Read:   resourceBackendRead,
		Delete: resourceBackendDelete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"listener_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"server_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"address": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceBackendCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.ElbV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	addOpts := backendmember.AddOpts{
		ServerId: d.Get("server_id").(string),
		Address:  d.Get("address").(string),
	}
	log.Printf("[DEBUG] Create Options: %#v", addOpts)

	listenerId := d.Get("listener_id").(string)
	job, err := backendmember.Add(client, listenerId, addOpts).ExtractJobResponse()
	if err != nil {
		return err
	}

	log.Printf("Waiting for backend to become active")
	if err := golangsdk.WaitForJobSuccess(client, job.URI, int(d.Timeout(schema.TimeoutCreate)/time.Second)); err != nil {
		return err
	}

	entity, err := golangsdk.GetJobEntity(client, job.URI, "members")

	if members, ok := entity.([]interface{}); ok {
		if len(members) > 0 {
			vmember := members[0]
			if member, ok := vmember.(map[string]interface{}); ok {
				if vid, ok := member["id"]; ok {
					if id, ok := vid.(string); ok {
						d.SetId(id)
						return resourceBackendRead(d, meta)
					}
				}
			}
		}
	}
	return fmt.Errorf("unexpected conversion error in resourceBackendCreate")
}

func resourceBackendRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.ElbV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	listenerId := d.Get("listener_id").(string)
	id := d.Id()
	backend, err := backendmember.Get(client, listenerId, id).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "backend member")
	}

	log.Printf("[DEBUG] Retrieved backend member %s: %#v", id, backend)

	b := backend[0]
	d.Set("server_id", b.ServerID)
	d.Set("address", b.ServerAddress)

	d.Set("region", config.GetRegion(d))

	return nil
}

func resourceBackendDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.ElbV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	log.Printf("[DEBUG] Deleting backend member %s", d.Id())
	listenerId := d.Get("listener_id").(string)
	id := d.Id()
	job, err := backendmember.Remove(client, listenerId, id).ExtractJobResponse()
	if err != nil {
		return err
	}

	log.Printf("Waiting for backend member %s to delete", id)

	if err := golangsdk.WaitForJobSuccess(client, job.URI, int(d.Timeout(schema.TimeoutDelete)/time.Second)); err != nil {
		return err
	}

	log.Printf("Successfully deleted backend member %s", id)
	return nil
}
