package elb

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/lbaas_v2/loadbalancers"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/ports"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/services/vpc"
)

func ResourceLoadBalancerV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceLoadBalancerV2Create,
		Read:   resourceLoadBalancerV2Read,
		Update: resourceLoadBalancerV2Update,
		Delete: resourceLoadBalancerV2Delete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"vip_subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"tenant_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"vip_address": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"vip_port_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"admin_state_up": {
				Type:         schema.TypeBool,
				Default:      true,
				Optional:     true,
				ValidateFunc: common.ValidateTrueOnly,
			},

			"loadbalancer_provider": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"security_group_ids": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
		},
	}
}

func resourceLoadBalancerV2Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	var lbProvider string
	if v, ok := d.GetOk("loadbalancer_provider"); ok {
		lbProvider = v.(string)
	}

	adminStateUp := d.Get("admin_state_up").(bool)
	createOpts := loadbalancers.CreateOpts{
		Name:         d.Get("name").(string),
		Description:  d.Get("description").(string),
		VipSubnetID:  d.Get("vip_subnet_id").(string),
		TenantID:     d.Get("tenant_id").(string),
		VipAddress:   d.Get("vip_address").(string),
		AdminStateUp: &adminStateUp,
		Provider:     lbProvider,
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	lb, err := loadbalancers.Create(networkingClient, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error creating LoadBalancer: %s", err)
	}

	// Wait for LoadBalancer to become active before continuing
	timeout := d.Timeout(schema.TimeoutCreate)
	err = waitForLBV2LoadBalancer(networkingClient, lb.ID, "ACTIVE", nil, timeout)
	if err != nil {
		return err
	}

	// Once the loadbalancer has been created, apply any requested security groups
	// to the port that was created behind the scenes.
	if err := resourceLoadBalancerV2SecurityGroups(networkingClient, lb.VipPortID, d); err != nil {
		return err
	}

	// If all has been successful, set the ID on the resource
	d.SetId(lb.ID)

	return resourceLoadBalancerV2Read(d, meta)
}

func resourceLoadBalancerV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	lb, err := loadbalancers.Get(networkingClient, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "loadbalancer")
	}

	log.Printf("[DEBUG] Retrieved loadbalancer %s: %#v", d.Id(), lb)

	d.Set("name", lb.Name)
	d.Set("description", lb.Description)
	d.Set("vip_subnet_id", lb.VipSubnetID)
	d.Set("tenant_id", lb.TenantID)
	d.Set("vip_address", lb.VipAddress)
	d.Set("vip_port_id", lb.VipPortID)
	d.Set("admin_state_up", lb.AdminStateUp)
	d.Set("loadbalancer_provider", lb.Provider)
	d.Set("region", config.GetRegion(d))

	// Get any security groups on the VIP Port
	if lb.VipPortID != "" {
		port, err := ports.Get(networkingClient, lb.VipPortID).Extract()
		if err != nil {
			return err
		}

		if err := d.Set("security_group_ids", port.SecurityGroups); err != nil {
			return fmt.Errorf("[DEBUG] Error saving security_group_ids to state for OpenTelekomCloud loadbalancer (%s): %s", d.Id(), err)
		}
	}

	return nil
}

func resourceLoadBalancerV2Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	var updateOpts loadbalancers.UpdateOpts
	if d.HasChange("name") {
		updateOpts.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		updateOpts.Description = d.Get("description").(string)
	}
	if d.HasChange("admin_state_up") {
		asu := d.Get("admin_state_up").(bool)
		updateOpts.AdminStateUp = &asu
	}

	// Wait for LoadBalancer to become active before continuing
	timeout := d.Timeout(schema.TimeoutUpdate)
	err = waitForLBV2LoadBalancer(networkingClient, d.Id(), "ACTIVE", nil, timeout)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Updating loadbalancer %s with options: %#v", d.Id(), updateOpts)
	err = resource.Retry(timeout, func() *resource.RetryError {
		_, err = loadbalancers.Update(networkingClient, d.Id(), updateOpts).Extract()
		if err != nil {
			return common.CheckForRetryableError(err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Unable to update loadbalancer %s: %s", d.Id(), err)
	}

	// Wait for LoadBalancer to become active before continuing
	err = waitForLBV2LoadBalancer(networkingClient, d.Id(), "ACTIVE", nil, timeout)
	if err != nil {
		return err
	}

	// Security Groups get updated separately
	if d.HasChange("security_group_ids") {
		vipPortID := d.Get("vip_port_id").(string)
		if err := resourceLoadBalancerV2SecurityGroups(networkingClient, vipPortID, d); err != nil {
			return err
		}
	}

	return resourceLoadBalancerV2Read(d, meta)
}

func resourceLoadBalancerV2Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	log.Printf("[DEBUG] Deleting loadbalancer %s", d.Id())
	timeout := d.Timeout(schema.TimeoutDelete)
	err = resource.Retry(timeout, func() *resource.RetryError {
		err = loadbalancers.Delete(networkingClient, d.Id()).ExtractErr()
		if err != nil {
			return common.CheckForRetryableError(err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Unable to delete loadbalancer %s: %s", d.Id(), err)
	}

	// Wait for LoadBalancer to become delete
	pending := []string{"PENDING_UPDATE", "PENDING_DELETE", "ACTIVE"}
	err = waitForLBV2LoadBalancer(networkingClient, d.Id(), "DELETED", pending, timeout)
	if err != nil {
		return err
	}

	return nil
}

func resourceLoadBalancerV2SecurityGroups(networkingClient *golangsdk.ServiceClient, vipPortID string, d *schema.ResourceData) error {
	if vipPortID != "" {
		if _, ok := d.GetOk("security_group_ids"); ok {
			securityGroups := vpc.ResourcePortSecurityGroupsV2(d)
			updateOpts := ports.UpdateOpts{
				SecurityGroups: &securityGroups,
			}

			log.Printf("[DEBUG] Adding security groups to loadbalancer "+
				"VIP Port %s: %#v", vipPortID, updateOpts)

			_, err := ports.Update(networkingClient, vipPortID, updateOpts).Extract()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
