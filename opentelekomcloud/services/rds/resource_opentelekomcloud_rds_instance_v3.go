package rds

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/common/tags"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/subnets"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/ports"
	tag "github.com/opentelekomcloud/gophertelekomcloud/openstack/rds/v1/tags"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rds/v3/backups"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rds/v3/configurations"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rds/v3/flavors"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rds/v3/instances"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceRdsInstanceV3() *schema.Resource {
	return &schema.Resource{
		Create: resourceRdsInstanceV3Create,
		Read:   resourceRdsInstanceV3Read,
		Update: resourceRdsInstanceV3Update,
		Delete: resourceRdsInstanceV3Delete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
		},

		CustomizeDiff: validateRDSv3Version("db"),

		Schema: map[string]*schema.Schema{
			"availability_zone": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"db": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"password": {
							Type:      schema.TypeString,
							Sensitive: true,
							Required:  true,
							ForceNew:  true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"version": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Computed: true,
							Optional: true,
							ForceNew: true,
						},
						"user_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"flavor": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: false,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"security_group_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"volume": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: false,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"size": {
							Type:     schema.TypeInt,
							Required: true,
							ForceNew: false,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"disk_encryption_id": {
							Type:     schema.TypeString,
							Computed: true,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"backup_strategy": {
				Type:     schema.TypeList,
				Computed: true,
				Optional: true,
				ForceNew: false,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"start_time": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: false,
						},
						"keep_days": {
							Type:     schema.TypeInt,
							Computed: true,
							Optional: true,
							ForceNew: false,
						},
					},
				},
			},
			"ha_replication_mode": {
				Type:     schema.TypeString,
				Computed: true,
				Optional: true,
				ForceNew: true,
			},
			"tag": {
				Type:          schema.TypeMap,
				Optional:      true,
				ValidateFunc:  common.ValidateTags,
				Deprecated:    "Please use `tags` instead",
				ConflictsWith: []string{"tags"},
			},
			"tags": {
				Type:          schema.TypeMap,
				Optional:      true,
				ValidateFunc:  common.ValidateTags,
				ConflictsWith: []string{"tag"},
			},
			"param_group_id": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"created": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"nodes": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"availability_zone": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"role": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"status": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"private_ips": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"public_ips": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: common.ValidateIP,
				},
			},
		},
	}
}

func resourceRDSDataStore(d *schema.ResourceData) *instances.Datastore {
	dataStoreRaw := d.Get("db").([]interface{})[0].(map[string]interface{})
	dataStore := instances.Datastore{
		Type:    dataStoreRaw["type"].(string),
		Version: dataStoreRaw["version"].(string),
	}
	return &dataStore
}

func resourceRDSVolume(d *schema.ResourceData) *instances.Volume {
	volumeRaw := d.Get("volume").([]interface{})[0].(map[string]interface{})
	volume := instances.Volume{
		Type: volumeRaw["type"].(string),
		Size: volumeRaw["size"].(int),
	}
	return &volume
}

func resourceRDSBackupStrategy(d *schema.ResourceData) *instances.BackupStrategy {
	backupStrategyRaw := d.Get("backup_strategy").([]interface{})
	if len(backupStrategyRaw) == 0 {
		return nil
	}
	backupStrategyInfo := backupStrategyRaw[0].(map[string]interface{})
	backupStrategy := instances.BackupStrategy{
		StartTime: backupStrategyInfo["start_time"].(string),
		KeepDays:  backupStrategyInfo["keep_days"].(int),
	}
	return &backupStrategy
}

func resourceRDSHa(d *schema.ResourceData) *instances.Ha {
	replicationMode := d.Get("ha_replication_mode").(string)
	if replicationMode == "" {
		return nil
	}
	ha := instances.Ha{
		Mode:            "Ha",
		ReplicationMode: replicationMode,
	}
	return &ha
}

func resourceRDSChangeMode() *instances.ChargeInfo {
	chargeInfo := instances.ChargeInfo{
		ChargeMode: "postPaid",
	}
	return &chargeInfo
}

func resourceRDSDbInfo(d *schema.ResourceData) map[string]interface{} {
	dbRaw := d.Get("db").([]interface{})[0].(map[string]interface{})
	return dbRaw
}

func resourceRDSAvailabilityZones(d *schema.ResourceData) string {
	azRaw := d.Get("availability_zone").([]interface{})
	zones := make([]string, 0)
	for _, v := range azRaw {
		zones = append(zones, v.(string))
	}
	zone := strings.Join(zones, ",")
	return zone
}

func resourceRdsInstanceV3Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating RDSv3 client: %s", err)
	}

	dbInfo := resourceRDSDbInfo(d)
	volumeInfo := d.Get("volume").([]interface{})[0].(map[string]interface{})
	dbPort := dbInfo["port"].(int)
	var dbPortString string
	if dbPort != 0 {
		dbPortString = strconv.Itoa(dbInfo["port"].(int))
	} else {
		dbPortString = ""
	}

	createOpts := instances.CreateRdsOpts{
		Name:             d.Get("name").(string),
		Datastore:        resourceRDSDataStore(d),
		Ha:               resourceRDSHa(d),
		ConfigurationId:  d.Get("param_group_id").(string),
		Port:             dbPortString,
		Password:         dbInfo["password"].(string),
		BackupStrategy:   resourceRDSBackupStrategy(d),
		DiskEncryptionId: volumeInfo["disk_encryption_id"].(string),
		FlavorRef:        d.Get("flavor").(string),
		Volume:           resourceRDSVolume(d),
		Region:           config.GetRegion(d),
		AvailabilityZone: resourceRDSAvailabilityZones(d),
		VpcId:            d.Get("vpc_id").(string),
		SubnetId:         d.Get("subnet_id").(string),
		SecurityGroupId:  d.Get("security_group_id").(string),
		ChargeInfo:       resourceRDSChangeMode(),
	}
	createResult := instances.Create(client, createOpts)
	r, err := createResult.Extract()
	if err != nil {
		return err
	}
	jobResponse, err := createResult.ExtractJobResponse()
	if err != nil {
		return err
	}

	timeout := d.Timeout(schema.TimeoutCreate)
	if err := instances.WaitForJobCompleted(client, int(timeout.Seconds()), jobResponse.JobID); err != nil {
		return err
	}

	d.SetId(r.Instance.Id)

	if common.HasFilledOpt(d, "tag") {
		rdsInstance, err := GetRdsInstance(client, r.Instance.Id)
		if err != nil {
			return err
		}
		nodeID := getMasterID(rdsInstance.Nodes)

		if nodeID == "" {
			log.Printf("[WARN] Error setting tag(key/value) of instance: %s", r.Instance.Id)
			return nil
		}
		tagClient, err := config.RdsTagV1Client(config.GetRegion(d))
		if err != nil {
			return fmt.Errorf("error creating OpenTelekomCloud RDSv1 tag client: %s", err)
		}
		tagMap := d.Get("tag").(map[string]interface{})
		log.Printf("[DEBUG] Setting tag(key/value): %v", tagMap)
		for key, val := range tagMap {
			tagOpts := tag.CreateOpts{
				Key:   key,
				Value: val.(string),
			}
			err = tag.Create(tagClient, nodeID, tagOpts).ExtractErr()
			if err != nil {
				log.Printf("[WARN] Error setting tag(key/value) of instance %s, err: %s", r.Instance.Id, err)
			}
		}
	}

	if common.HasFilledOpt(d, "tags") {
		tagRaw := d.Get("tags").(map[string]interface{})
		if len(tagRaw) > 0 {
			tagList := common.ExpandResourceTags(tagRaw)
			if err := tags.Create(client, "instances", r.Instance.Id, tagList).ExtractErr(); err != nil {
				return fmt.Errorf("error setting tags of RDSv3 instance: %w", err)
			}
		}
	}

	ip := getPublicIP(d)
	if ip != "" {
		if err = resourceRdsInstanceV3Read(d, meta); err != nil {
			return err
		}
		nw, err := config.NetworkingV2Client(config.GetRegion(d))
		if err != nil {
			return err
		}
		subnetID, err := getSubnetSubnetID(d, config)
		if err != nil {
			return err
		}
		if err := assignEipToInstance(nw, ip, getPrivateIP(d), subnetID); err != nil {
			log.Printf("[WARN] failed to assign public IP: %s", err)
		}
	}

	return resourceRdsInstanceV3Read(d, meta)
}

func GetRdsInstance(rdsClient *golangsdk.ServiceClient, rdsId string) (*instances.RdsInstanceResponse, error) {
	listOpts := instances.ListRdsInstanceOpts{
		Id: rdsId,
	}
	allPages, err := instances.List(rdsClient, listOpts).AllPages()
	if err != nil {
		return nil, err
	}

	n, err := instances.ExtractRdsInstances(allPages)
	if err != nil {
		return nil, err
	}
	if len(n.Instances) == 0 {
		return nil, nil
	}
	return &n.Instances[0], nil
}

func getPrivateIP(d *schema.ResourceData) string {
	return d.Get("private_ips").([]interface{})[0].(string)
}

func getPublicIP(d *schema.ResourceData) string {
	publicIpRaw := d.Get("public_ips").([]interface{})
	if len(publicIpRaw) > 0 {
		return publicIpRaw[0].(string)
	}
	return ""
}

func findFloatingIP(client *golangsdk.ServiceClient, address string) (id string, err error) {
	var opts = floatingips.ListOpts{FloatingIP: address}

	pgFIP, err := floatingips.List(client, opts).AllPages()
	if err != nil {
		return
	}
	floatingIPs, err := floatingips.ExtractFloatingIPs(pgFIP)
	if err != nil {
		return
	}
	if len(floatingIPs) == 0 {
		return
	}

	for _, ip := range floatingIPs {
		if address != ip.FloatingIP {
			continue
		}
		return floatingIPs[0].ID, nil
	}
	return
}

// find assigned port
func findPort(client *golangsdk.ServiceClient, privateIP string, subnetID string) (id string, err error) {
	pg, err := ports.List(client, nil).AllPages()
	if err != nil {
		return
	}

	portList, err := ports.ExtractPorts(pg)
	if err != nil {
		return
	}

	for _, port := range portList {
		address := port.FixedIPs[0]
		if address.IPAddress == privateIP && address.SubnetID == subnetID {
			id = port.ID
			return
		}
	}
	return
}

func assignEipToInstance(client *golangsdk.ServiceClient, publicIP, privateIP, subnetID string) error {
	portID, err := findPort(client, privateIP, subnetID)
	if err != nil {
		return err
	}

	ipID, err := findFloatingIP(client, publicIP)
	if err != nil {
		return err
	}
	return floatingips.Update(client, ipID, floatingips.UpdateOpts{PortID: &portID}).Err
}

func getSubnetSubnetID(d *schema.ResourceData, config *cfg.Config) (id string, err error) {
	subnetClient, err := config.NetworkingV1Client(config.GetRegion(d))
	if err != nil {
		err = fmt.Errorf("[WARN] Failed to create VPC client")
		return
	}
	sn, err := subnets.Get(subnetClient, d.Get("subnet_id").(string)).Extract()
	if err != nil {
		return
	}
	id = sn.SubnetID
	return
}

func unAssignEipFromInstance(client *golangsdk.ServiceClient, oldPublicIP string) error {
	ipID, err := findFloatingIP(client, oldPublicIP)
	if err != nil {
		return err
	}
	return floatingips.Update(client, ipID, floatingips.UpdateOpts{PortID: nil}).Err
}

func resourceRdsInstanceV3Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud RDSv3 Client: %s", err)
	}
	var updateBackupOpts backups.UpdateOpts

	if d.HasChange("backup_strategy") {
		backupRaw := resourceRDSBackupStrategy(d)
		updateBackupOpts.KeepDays = &backupRaw.KeepDays
		updateBackupOpts.StartTime = backupRaw.StartTime
		updateBackupOpts.Period = "1,2,3,4,5,6,7"
		log.Printf("[DEBUG] updateOpts: %#v", updateBackupOpts)

		if err = backups.Update(client, d.Id(), updateBackupOpts).ExtractErr(); err != nil {
			return fmt.Errorf("error updating OpenTelekomCloud RDSv3 Instance: %s", err)
		}
	}

	// Fetching node id
	var nodeID string
	v, err := GetRdsInstance(client, d.Id())
	if err != nil {
		return err
	}
	nodeID = getMasterID(v.Nodes)
	if nodeID == "" {
		log.Printf("[WARN] Error fetching node id of instance:%s", d.Id())
		return nil
	}

	if d.HasChange("tag") {
		oldTagRaw, newTagRaw := d.GetChange("tag")
		oldTag := oldTagRaw.(map[string]interface{})
		newTag := newTagRaw.(map[string]interface{})
		create, remove := diffTagsRDS(oldTag, newTag)
		tagClient, err := config.RdsTagV1Client(config.GetRegion(d))
		if err != nil {
			return fmt.Errorf("Error creating OpenTelekomCloud RDSv3 tag client: %s ", err)
		}

		if len(remove) > 0 {
			for _, opts := range remove {
				err = tag.Delete(tagClient, nodeID, opts).ExtractErr()
				if err != nil {
					log.Printf("[WARN] Error deleting tag(key/value) of instance: %s, err: %s", d.Id(), err)
				}
			}
		}
		if len(create) > 0 {
			for _, opts := range create {
				err = tag.Create(tagClient, nodeID, opts).ExtractErr()
				if err != nil {
					log.Printf("[WARN] Error setting tag(key/value) of instance: %s, err: %s", d.Id(), err)
				}
			}
		}
	}
	if d.HasChange("tags") {
		if err := common.UpdateResourceTags(client, d, "instances", d.Id()); err != nil {
			return fmt.Errorf("error updating tags of RDSv3 instance %s: %s", d.Id(), err)
		}
	}

	if d.HasChange("flavor") {
		_, newFlavor := d.GetChange("flavor")

		// Fetch flavor id
		db := resourceRDSDbInfo(d)
		datastoreType := db["type"].(string)
		datastoreVersion := db["version"].(string)

		dbFlavorsOpts := flavors.DbFlavorsOpts{
			Versionname: datastoreVersion,
		}
		flavorsPages, err := flavors.List(client, dbFlavorsOpts, datastoreType).AllPages()
		if err != nil {
			return fmt.Errorf("unable to retrieve flavors all pages: %s", err)
		}
		flavorsList, err := flavors.ExtractDbFlavors(flavorsPages)
		if err != nil {
			return err
		}
		if len(flavorsList.Flavorslist) < 1 {
			return fmt.Errorf("no flavors returned")
		}
		var rdsFlavor flavors.Flavors
		for _, flavor := range flavorsList.Flavorslist {
			if flavor.Speccode == newFlavor.(string) {
				rdsFlavor = flavor
				break
			}
		}
		updateFlavorOpts := instances.ResizeFlavorOpts{
			ResizeFlavor: &instances.SpecCode{
				Speccode: rdsFlavor.Speccode,
			},
		}

		log.Printf("Update flavor could be done only in status `available`")
		if err := instances.WaitForStateAvailable(client, 1200, d.Id()); err != nil {
			log.Printf("Status available wasn't present")
		}

		log.Printf("[DEBUG] Update flavor: %s", newFlavor.(string))
		_, err = instances.Resize(client, updateFlavorOpts, d.Id()).Extract()
		if err != nil {
			return fmt.Errorf("error updating instance Flavor from result: %s", err)
		}

		log.Printf("Waiting for RDSv3 become in status `available`")
		if err := instances.WaitForStateAvailable(client, 1200, d.Id()); err != nil {
			log.Printf("Status available wasn't present")
		}

		log.Printf("[DEBUG] Successfully updated instance %s flavor: %s", d.Id(), d.Get("flavor").(string))
	}

	if d.HasChange("volume") {
		_, newVolume := d.GetChange("volume")
		volume := make(map[string]interface{})
		volumeRaw := newVolume.([]interface{})
		log.Printf("[DEBUG] volumeRaw: %+v", volumeRaw)
		if len(volumeRaw) == 1 {
			if m, ok := volumeRaw[0].(map[string]interface{}); ok {
				volume["size"] = m["size"].(int)
			}
		}
		log.Printf("[DEBUG] volume: %+v", volume)
		updateOpts := instances.EnlargeVolumeRdsOpts{
			EnlargeVolume: &instances.EnlargeVolumeSize{
				Size: volume["size"].(int),
			},
		}

		log.Printf("Update volume size could be done only in status `available`")
		if err := instances.WaitForStateAvailable(client, 1200, d.Id()); err != nil {
			log.Printf("Status available wasn't present")
		}

		updateResult, err := instances.EnlargeVolume(client, updateOpts, d.Id()).ExtractJobResponse()
		if err != nil {
			return fmt.Errorf("error updating instance volume from result: %s", err)
		}
		timeout := d.Timeout(schema.TimeoutCreate)
		if err := instances.WaitForJobCompleted(client, int(timeout.Seconds()), updateResult.JobID); err != nil {
			return err
		}

		log.Printf("[DEBUG] Successfully updated instance %s volume: %+v", d.Id(), volume)
	}

	if d.HasChange("public_ips") {
		nwClient, err := config.NetworkingV2Client(config.GetRegion(d))
		oldPublicIps, newPublicIps := d.GetChange("public_ips")
		oldIPs := oldPublicIps.([]interface{})
		newIPs := newPublicIps.([]interface{})
		switch len(newIPs) {
		case 0:
			err = unAssignEipFromInstance(nwClient, oldIPs[0].(string)) // if it become 0, it was 1 before
			break
		case 1:
			if len(oldIPs) > 0 {
				err = unAssignEipFromInstance(nwClient, oldIPs[0].(string))
				if err != nil {
					return err
				}
			}
			privateIP := getPrivateIP(d)
			subnetID, err := getSubnetSubnetID(d, config)
			if err != nil {
				return err
			}
			err = assignEipToInstance(nwClient, newIPs[0].(string), privateIP, subnetID)
			break
		default:
			return fmt.Errorf("RDS instance can't have more than one public IP")
		}
	}

	if d.HasChange("param_group_id") {
		newParamGroupID := d.Get("param_group_id").(string)
		if len(newParamGroupID) == 0 {
			return fmt.Errorf("you can't remove `param_group_id` without recreation")
		}
		applyOpts := configurations.ApplyOpts{
			InstanceIDs: []string{
				d.Id(),
			},
		}
		if err := configurations.Apply(client, newParamGroupID, applyOpts).Err; err != nil {
			return fmt.Errorf("error during apply new configuration: %s", err)
		}
	}

	return resourceRdsInstanceV3Read(d, meta)
}

func getMasterID(nodes []instances.Nodes) (nodeID string) {
	for _, node := range nodes {
		if node.Role == "master" {
			nodeID = node.Id
		}
	}
	return
}

func resourceRdsInstanceV3Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating RDSv3 client: %s", err)
	}

	rdsInstance, err := GetRdsInstance(client, d.Id())
	if err != nil {
		return fmt.Errorf("error fetching RDS instance: %s", err)
	}
	if rdsInstance == nil {
		d.SetId("")
		return nil
	}

	me := multierror.Append(nil,
		d.Set("flavor", rdsInstance.FlavorRef),
		d.Set("name", rdsInstance.Name),
		d.Set("security_group_id", rdsInstance.SecurityGroupId),
		d.Set("subnet_id", rdsInstance.SubnetId),
		d.Set("vpc_id", rdsInstance.VpcId),
		d.Set("created", rdsInstance.Created),
		d.Set("ha_replication_mode", rdsInstance.Ha.ReplicationMode),
	)

	if me.ErrorOrNil() != nil {
		return err
	}

	var nodesList []map[string]interface{}
	for _, nodeObj := range rdsInstance.Nodes {
		node := make(map[string]interface{})
		node["id"] = nodeObj.Id
		node["role"] = nodeObj.Role
		node["name"] = nodeObj.Name
		node["availability_zone"] = nodeObj.AvailabilityZone
		node["status"] = nodeObj.Status
		nodesList = append(nodesList, node)
	}
	if err := d.Set("nodes", nodesList); err != nil {
		return fmt.Errorf("error setting node list: %s", err)
	}

	var backupStrategyList []map[string]interface{}
	backupStrategy := make(map[string]interface{})
	backupStrategy["start_time"] = rdsInstance.BackupStrategy.StartTime
	backupStrategy["keep_days"] = rdsInstance.BackupStrategy.KeepDays
	backupStrategyList = append(backupStrategyList, backupStrategy)
	if err := d.Set("backup_strategy", backupStrategyList); err != nil {
		return fmt.Errorf("error setting backup strategy: %s", err)
	}

	var volumeList []map[string]interface{}
	volume := make(map[string]interface{})
	volume["size"] = rdsInstance.Volume.Size
	volume["type"] = rdsInstance.Volume.Type
	volume["disk_encryption_id"] = rdsInstance.DiskEncryptionId
	volumeList = append(volumeList, volume)
	if err = d.Set("volume", volumeList); err != nil {
		return err
	}

	dbRaw := d.Get("db").([]interface{})
	dbInfo := make(map[string]interface{})
	if len(dbRaw) != 0 {
		dbInfo = dbRaw[0].(map[string]interface{})
	}
	dbInfo["type"] = rdsInstance.DataStore.Type
	dbInfo["version"] = rdsInstance.DataStore.Version
	dbInfo["port"] = rdsInstance.Port
	dbInfo["user_name"] = rdsInstance.DbUserName
	dbList := []interface{}{dbInfo}
	if err = d.Set("db", dbList); err != nil {
		return err
	}

	if err = d.Set("private_ips", rdsInstance.PrivateIps); err != nil {
		return err
	}

	publicIp := getPublicIP(d)
	if publicIp != "" {
		if err = d.Set("public_ips", []string{publicIp}); err != nil {
			return err
		}
	}

	var tagParamName string
	// set instance tags
	if _, ok := d.GetOk("tags"); ok {
		tagParamName = "tags"
	} else if _, ok := d.GetOk("tag"); ok {
		tagParamName = "tag"
	}
	if tagParamName == "tag" {
		// set instance tag
		var nodeID string
		nodes := d.Get("nodes").([]interface{})
		for _, node := range nodes {
			nodeObj := node.(map[string]interface{})
			if nodeObj["role"].(string) == "master" {
				nodeID = nodeObj["id"].(string)
			}
		}

		if nodeID == "" {
			log.Printf("[WARN] Error fetching node id of instance: %s", d.Id())
			return nil
		}
		tagClient, err := config.RdsTagV1Client(config.GetRegion(d))
		if err != nil {
			return fmt.Errorf("error creating OpenTelekomCloud rds tag client: %#v", err)
		}
		tagList, err := tag.Get(tagClient, nodeID).Extract()
		if err != nil {
			return fmt.Errorf("error fetching OpenTelekomCloud rds instance tags: %s", err)
		}
		tagMap := make(map[string]string)
		for _, val := range tagList.Tags {
			tagMap[val.Key] = val.Value
		}
		if err := d.Set("tag", tagMap); err != nil {
			return fmt.Errorf("[DEBUG] Error saving tag to state for OpenTelekomCloud rds instance (%s): %s", d.Id(), err)
		}
	} else if tagParamName == "tags" {
		tagsMap := common.TagsToMap(rdsInstance.Tags)
		if err := d.Set("tags", tagsMap); err != nil {
			return fmt.Errorf("error saving tags for OpenTelekomCloud RDSv3 instance: %s", err)
		}
	}

	return nil
}

func resourceRdsInstanceV3Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.RdsV3Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud RDSv3 client: %s", err)
	}

	log.Printf("[DEBUG] Deleting Instance %s", d.Id())

	_, err = instances.Delete(client, d.Id()).Extract()
	if err != nil {
		return fmt.Errorf("eror deleting OpenTelekomCloud RDSv3 instance: %s", err)
	}

	d.SetId("")
	return nil
}

func validateRDSv3Version(argumentName string) schema.CustomizeDiffFunc {
	return func(d *schema.ResourceDiff, meta interface{}) error {
		config, ok := meta.(*cfg.Config)
		if !ok {
			return fmt.Errorf("error retreiving configuration: can't convert %v to Config", meta)
		}

		rdsClient, err := config.RdsV3Client(config.GetRegion(d))
		if err != nil {
			return fmt.Errorf("error creating OpenTelekomCloud RDSv3 Client: %s", err)
		}

		dataStoreInfo := d.Get(argumentName).([]interface{})[0].(map[string]interface{})
		datastoreVersions, err := getRdsV3VersionList(rdsClient, dataStoreInfo["type"].(string))
		if err != nil {
			return fmt.Errorf("unable to get datastore versions: %s", err)
		}

		var matches = false
		for _, datastore := range datastoreVersions {
			if datastore == dataStoreInfo["version"] {
				matches = true
				break
			}
		}
		if !matches {
			return fmt.Errorf("can't find version `%s`", dataStoreInfo["version"])
		}

		return nil
	}
}
