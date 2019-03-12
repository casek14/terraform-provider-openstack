package openstack

import (
	"fmt"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/lbaas_v2/loadbalancers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"log"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
)

func resourceNetworkingFloatingIPAssociateV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetworkingFloatingIPAssociateV2Create,
		Read:   resourceNetworkingFloatingIPAssociateV2Read,
		Delete: resourceNetworkingFloatingIPAssociateV2Delete,
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

			"floating_ip": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"port_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceNetworkingFloatingIPAssociateV2Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	networkingClient, err := config.networkingV2Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack network client: %s", err)
	}

	floatingIP := d.Get("floating_ip").(string)
	portID := d.Get("port_id").(string)

	fipID, err := networkingFloatingIPV2ID(networkingClient, floatingIP)
	if err != nil {
		return fmt.Errorf("Unable to get ID of openstack_networking_floatingip_v2: %s", err)
	}

	var fixedIp string = ""
	// Get fixed_ip of the port, if fixed ip does not exist get it from LB
	port,err := ports.Get(networkingClient, portID).Extract()
	if err != nil {
		return fmt.Errorf("Unable to get port %s, - %s",portID,err)
	}

	portIps := port.FixedIPs
	if len(portIps) > 0 {
		fixedIp = portIps[0].IPAddress
		log.Printf("[DEBUG] FIXED IP GET FROM PORT ###############" )
	}else {
		// get fixed ip from LB
		lbClient, err := chooseLBV2Client(d, config)
		if err != nil {
			return fmt.Errorf("Error creating OpenStack networking client: %s", err)
		}

		filter := loadbalancers.ListOpts{VipPortID: portID}
		allPages, err := loadbalancers.List(lbClient, filter).AllPages()
		if err != nil {
			return fmt.Errorf("Error listing loadbalancers: %s", err)
		}
		loadbalancers, err := loadbalancers.ExtractLoadBalancers(allPages)

		if len(loadbalancers) == 1 {
			fixedIp = loadbalancers[0].VipAddress
			log.Printf("[DEBUG] FIXED IP GET FROM LOADBALANCER ###############" )
			}

	}


	updateOpts := floatingips.UpdateOpts{
		PortID: &portID,
		FixedIP: fixedIp,
	}

	log.Printf("[DEBUG] openstack_networking_floatingip_associate_v2 create options: %#v", updateOpts)
	_, err = floatingips.Update(networkingClient, fipID, updateOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error associating openstack_networking_floatingip_v2 %s to openstack_networking_port_v2 %s: %s",
			fipID, portID, err)
	}

	d.SetId(fipID)

	log.Printf("[DEBUG] Created association between openstack_networking_floatingip_v2 %s and openstack_networking_port_v2 %s",
		fipID, portID)
	return resourceNetworkingFloatingIPAssociateV2Read(d, meta)
}

func resourceNetworkingFloatingIPAssociateV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	networkingClient, err := config.networkingV2Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack network client: %s", err)
	}

	fip, err := floatingips.Get(networkingClient, d.Id()).Extract()
	if err != nil {
		return CheckDeleted(d, err, "Error getting openstack_networking_floatingip_v2")
	}

	log.Printf("[DEBUG] Retrieved openstack_networking_floatingip_v2 %s: %#v", d.Id(), fip)

	d.Set("floating_ip", fip.FloatingIP)
	d.Set("port_id", fip.PortID)
	d.Set("region", GetRegion(d, config))

	return nil
}

func resourceNetworkingFloatingIPAssociateV2Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	networkingClient, err := config.networkingV2Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack network client: %s", err)
	}

	portID := d.Get("port_id").(string)
	updateOpts := floatingips.UpdateOpts{
		PortID: new(string),
	}

	log.Printf("[DEBUG] openstack_networking_floatingip_v2 disassociating options: %#v", updateOpts)
	_, err = floatingips.Update(networkingClient, d.Id(), updateOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error disassociating openstack_networking_floatingip_v2 %s from openstack_networking_port_v2 %s: %s",
			d.Id(), portID, err)
	}

	return nil
}
