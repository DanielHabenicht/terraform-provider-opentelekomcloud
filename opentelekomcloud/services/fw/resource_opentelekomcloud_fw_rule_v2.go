package fw

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/fwaas_v2/policies"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/fwaas_v2/rules"
	"github.com/opentelekomcloud/gophertelekomcloud/pagination"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceFWRuleV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceFWRuleV2Create,
		Read:   resourceFWRuleV2Read,
		Update: resourceFWRuleV2Update,
		Delete: resourceFWRuleV2Delete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
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
			"protocol": {
				Type:     schema.TypeString,
				Required: true,
			},
			"action": {
				Type:     schema.TypeString,
				Required: true,
			},
			"ip_version": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  4,
			},
			"source_ip_address": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"destination_ip_address": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"source_port": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"destination_port": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"tenant_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"value_specs": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceFWRuleV2Create(d *schema.ResourceData, meta interface{}) error {

	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	enabled := d.Get("enabled").(bool)
	ipVersion := resourceFWRuleV2DetermineIPVersion(d.Get("ip_version").(int))
	protocol := resourceFWRuleV2DetermineProtocol(d.Get("protocol").(string))

	ruleConfiguration := RuleCreateOpts{
		rules.CreateOpts{
			Name:                 d.Get("name").(string),
			Description:          d.Get("description").(string),
			Protocol:             protocol,
			Action:               d.Get("action").(string),
			IPVersion:            ipVersion,
			SourceIPAddress:      d.Get("source_ip_address").(string),
			DestinationIPAddress: d.Get("destination_ip_address").(string),
			SourcePort:           d.Get("source_port").(string),
			DestinationPort:      d.Get("destination_port").(string),
			Enabled:              &enabled,
			TenantID:             d.Get("tenant_id").(string),
		},
		common.MapValueSpecs(d),
	}

	log.Printf("[DEBUG] Create firewall rule: %#v", ruleConfiguration)

	rule, err := rules.Create(networkingClient, ruleConfiguration).Extract()

	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Firewall rule with id %s : %#v", rule.ID, rule)

	d.SetId(rule.ID)

	return resourceFWRuleV2Read(d, meta)
}

func resourceFWRuleV2Read(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] Retrieve information about firewall rule: %s", d.Id())

	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	rule, err := rules.Get(networkingClient, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "FW rule")
	}

	log.Printf("[DEBUG] Read OpenTelekomCloud Firewall Rule %s: %#v", d.Id(), rule)

	d.Set("action", rule.Action)
	d.Set("name", rule.Name)
	d.Set("description", rule.Description)
	d.Set("ip_version", rule.IPVersion)
	d.Set("source_ip_address", rule.SourceIPAddress)
	d.Set("destination_ip_address", rule.DestinationIPAddress)
	d.Set("source_port", rule.SourcePort)
	d.Set("destination_port", rule.DestinationPort)
	d.Set("enabled", rule.Enabled)

	if rule.Protocol == "" {
		d.Set("protocol", "any")
	} else {
		d.Set("protocol", rule.Protocol)
	}

	d.Set("region", config.GetRegion(d))

	return nil
}

func resourceFWRuleV2Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	var updateOpts rules.UpdateOpts
	if d.HasChange("name") {
		name := d.Get("name").(string)
		updateOpts.Name = &name
	}
	if d.HasChange("description") {
		description := d.Get("description").(string)
		updateOpts.Description = &description
	}
	if d.HasChange("protocol") {
		protocol := d.Get("protocol").(string)
		updateOpts.Protocol = &protocol
	}
	if d.HasChange("action") {
		action := d.Get("action").(string)
		updateOpts.Action = &action
	}
	if d.HasChange("ip_version") {
		ipVersion := resourceFWRuleV2DetermineIPVersion(d.Get("ip_version").(int))
		updateOpts.IPVersion = &ipVersion
	}
	if d.HasChange("source_ip_address") {
		sourceIPAddress := d.Get("source_ip_address").(string)
		updateOpts.SourceIPAddress = &sourceIPAddress
	}
	if d.HasChange("source_port") {
		sourcePort := d.Get("source_port").(string)
		updateOpts.SourcePort = &sourcePort
	}
	if d.HasChange("destination_ip_address") {
		destinationIPAddress := d.Get("destination_ip_address").(string)
		updateOpts.DestinationIPAddress = &destinationIPAddress
	}
	if d.HasChange("destination_port") {
		destinationPort := d.Get("destination_port").(string)
		updateOpts.DestinationPort = &destinationPort
	}
	if d.HasChange("enabled") {
		enabled := d.Get("enabled").(bool)
		updateOpts.Enabled = &enabled
	}

	log.Printf("[DEBUG] Updating firewall rules: %#v", updateOpts)
	err = rules.Update(networkingClient, d.Id(), updateOpts).Err
	if err != nil {
		return err
	}

	return resourceFWRuleV2Read(d, meta)
}

func resourceFWRuleV2Delete(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[DEBUG] Destroy firewall rule: %s", d.Id())

	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	rule, err := rules.Get(networkingClient, d.Id()).Extract()
	if err != nil {
		return err
	}

	policyID, err := assignedPolicyID(networkingClient, rule.ID)
	if err != nil {
		return err
	}
	if policyID != "" {
		_, err := policies.RemoveRule(networkingClient, policyID, rule.ID).Extract()
		if err != nil {
			return err
		}
	}

	return rules.Delete(networkingClient, d.Id()).Err
}

func assignedPolicyID(nwClient *golangsdk.ServiceClient, ruleID string) (string, error) {
	pager := policies.List(nwClient, policies.ListOpts{})
	policyID := ""
	err := pager.EachPage(func(page pagination.Page) (b bool, err error) {
		policyList, err := policies.ExtractPolicies(page)
		if err != nil {
			return false, err
		}
		for _, policy := range policyList {
			for _, rule := range policy.Rules {
				if rule == ruleID {
					policyID = policy.ID
					return false, nil
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return policyID, nil
}

func resourceFWRuleV2DetermineIPVersion(ipv int) golangsdk.IPVersion {
	// Determine the IP Version
	var ipVersion golangsdk.IPVersion
	switch ipv {
	case 4:
		ipVersion = golangsdk.IPv4
	case 6:
		ipVersion = golangsdk.IPv6
	}

	return ipVersion
}

func resourceFWRuleV2DetermineProtocol(p string) rules.Protocol {
	var protocol rules.Protocol
	switch p {
	case "any":
		protocol = rules.ProtocolAny
	case "icmp":
		protocol = rules.ProtocolICMP
	case "tcp":
		protocol = rules.ProtocolTCP
	case "udp":
		protocol = rules.ProtocolUDP
	}

	return protocol
}
