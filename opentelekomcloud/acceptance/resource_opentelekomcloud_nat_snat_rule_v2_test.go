package acceptance

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/snatrules"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/networks"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/subnets"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func TestAccNatSnatRule_basic(t *testing.T) {
	var fip floatingips.FloatingIP
	var network networks.Network
	var router routers.Router
	var subnet subnets.Subnet

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckNatV2SnatRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNatV2SnatRule_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNetworkingV2NetworkExists("opentelekomcloud_networking_network_v2.network_1", &network),
					testAccCheckNetworkingV2SubnetExists("opentelekomcloud_networking_subnet_v2.subnet_1", &subnet),
					testAccCheckNetworkingV2RouterExists("opentelekomcloud_networking_router_v2.router_1", &router),
					testAccCheckNetworkingV2FloatingIPExists("opentelekomcloud_networking_floatingip_v2.fip_1", &fip),
					testAccCheckNetworkingV2RouterInterfaceExists("opentelekomcloud_networking_router_interface_v2.int_1"),
					testAccCheckNatV2GatewayExists("opentelekomcloud_nat_gateway_v2.nat_1"),
					testAccCheckNatV2SnatRuleExists("opentelekomcloud_nat_snat_rule_v2.snat_1"),
				),
			},
		},
	})
}

func testAccCheckNatV2SnatRuleDestroy(s *terraform.State) error {
	config := testAccProvider.Meta().(*cfg.Config)
	natClient, err := config.NatV2Client(OS_REGION_NAME)
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud nat client: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "opentelekomcloud_nat_snat_rule_v2" {
			continue
		}

		_, err := snatrules.Get(natClient, rs.Primary.ID).Extract()
		if err == nil {
			return fmt.Errorf("Snat rule still exists")
		}
	}

	return nil
}

func testAccCheckNatV2SnatRuleExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set")
		}

		config := testAccProvider.Meta().(*cfg.Config)
		natClient, err := config.NatV2Client(OS_REGION_NAME)
		if err != nil {
			return fmt.Errorf("Error creating OpenTelekomCloud nat client: %s", err)
		}

		found, err := snatrules.Get(natClient, rs.Primary.ID).Extract()
		if err != nil {
			return err
		}

		if found.ID != rs.Primary.ID {
			return fmt.Errorf("Snat rule not found")
		}

		return nil
	}
}

const testAccNatV2SnatRule_basic = `
resource "opentelekomcloud_networking_router_v2" "router_1" {
  name = "router_1"
  admin_state_up = "true"
}

resource "opentelekomcloud_networking_network_v2" "network_1" {
  name = "network_1"
  admin_state_up = "true"
}

resource "opentelekomcloud_networking_subnet_v2" "subnet_1" {
  cidr = "192.168.0.0/16"
  ip_version = 4
  network_id = opentelekomcloud_networking_network_v2.network_1.id
}

resource "opentelekomcloud_networking_router_interface_v2" "int_1" {
  subnet_id = opentelekomcloud_networking_subnet_v2.subnet_1.id
  router_id = opentelekomcloud_networking_router_v2.router_1.id
}

resource "opentelekomcloud_networking_floatingip_v2" "fip_1" {
}

resource "opentelekomcloud_nat_gateway_v2" "nat_1" {
  name   = "nat_1"
  description = "test for terraform"
  spec = "1"
  internal_network_id = opentelekomcloud_networking_network_v2.network_1.id
  router_id = opentelekomcloud_networking_router_v2.router_1.id
  depends_on = ["opentelekomcloud_networking_router_interface_v2.int_1"]
}

resource "opentelekomcloud_nat_snat_rule_v2" "snat_1" {
  nat_gateway_id = opentelekomcloud_nat_gateway_v2.nat_1.id
  floating_ip_id = opentelekomcloud_networking_floatingip_v2.fip_1.id
  cidr = "192.168.0.0/24"
  source_type = 0
}
`
