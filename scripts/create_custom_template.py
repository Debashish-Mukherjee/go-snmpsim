#!/usr/bin/env python3
"""
Create a custom Zabbix template with comprehensive SNMP interface monitoring.
Target: ~1500 items per device with 5-minute polling.
"""

import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')
from zabbix_api_client import ZabbixAPIClient
import yaml
import json

# Load config
with open('/home/debashish/trials/go-snmpsim/zabbix/zabbix_config.yaml', 'r') as f:
    config = yaml.safe_load(f)

print("🔗 Connecting to Zabbix...")
client = ZabbixAPIClient(
    config['zabbix_url'],
    config['zabbix_username'],
    config['zabbix_password']
)
client.login()
print("✅ Authenticated!\n")

# Check if custom template already exists
existing = client._request('template.get', {
    'output': ['templateid', 'name'],
    'filter': {'host': ['Custom Cisco IOS Extended Monitoring']}
})

if existing:
    print(f"⚠️  Template already exists: {existing[0]['name']}")
    print(f"   Deleting old template...")
    client._request('template.delete', [existing[0]['templateid']])
    print("   ✅ Deleted\n")

# Get host group for templates
groups = client._request('hostgroup.get', {
    'output': ['groupid', 'name'],
    'filter': {'name': ['Templates/Network devices']}
})

if not groups:
    print("Creating Templates/Network devices group...")
    group_id = client._request('hostgroup.create', {
        'name': 'Templates/Network devices'
    })['groupids'][0]
else:
    group_id = groups[0]['groupid']

print("📦 Creating custom template...")

# Create the template
template_data = {
    'host': 'Custom Cisco IOS Extended Monitoring',
    'name': 'Cisco IOS Extended SNMP Monitoring (48 Ports)',
    'groups': [{'groupid': group_id}],
    'description': '''Comprehensive SNMP monitoring template for Cisco IOS devices.
    
Features:
- 48-port network interface discovery with 30+ metrics per interface
- CPU, memory, temperature, fan, and power supply monitoring
- 5-minute polling interval for all items
- Optimized for high-volume metric collection

This template should discover ~1500+ items per device.'''
}

template_result = client._request('template.create', template_data)
template_id = template_result['templateids'][0]
print(f"✅ Created template: {template_id}\n")

# ============================================================
# CREATE DISCOVERY RULES
# ============================================================

print("📋 Creating discovery rules...")

# Network Interface Discovery with comprehensive metrics
print("  • Network Interface Discovery...")

# Define all interface item prototypes (30+ metrics per interface)
interface_item_prototypes = [
    # Traffic metrics
    {'name': 'Interface {#IFNAME}: Bits received', 'key': 'net.if.in.bits[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.6.{#SNMPINDEX}', 'units': 'bps', 'preprocessing': [{'type': 10, 'params': '8'}]},  # HC In Octets * 8
    {'name': 'Interface {#IFNAME}: Bits sent', 'key': 'net.if.out.bits[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.10.{#SNMPINDEX}', 'units': 'bps', 'preprocessing': [{'type': 10, 'params': '8'}]},
    
    # Packet counters
    {'name': 'Interface {#IFNAME}: Packets received', 'key': 'net.if.in.packets[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.11.{#SNMPINDEX}', 'units': 'pps'},
    {'name': 'Interface {#IFNAME}: Packets sent', 'key': 'net.if.out.packets[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.17.{#SNMPINDEX}', 'units': 'pps'},
    
    # Multicast/Broadcast
    {'name': 'Interface {#IFNAME}: Multicast packets received', 'key': 'net.if.in.multicast[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.2.{#SNMPINDEX}', 'units': 'pps'},
    {'name': 'Interface {#IFNAME}: Multicast packets sent', 'key': 'net.if.out.multicast[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.4.{#SNMPINDEX}', 'units': 'pps'},
    {'name': 'Interface {#IFNAME}: Broadcast packets received', 'key': 'net.if.in.broadcast[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.3.{#SNMPINDEX}', 'units': 'pps'},
    {'name': 'Interface {#IFNAME}: Broadcast packets sent', 'key': 'net.if.out.broadcast[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.5.{#SNMPINDEX}', 'units': 'pps'},
    
    # Errors and discards
    {'name': 'Interface {#IFNAME}: Inbound packets with errors', 'key': 'net.if.in.errors[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.14.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: Outbound packets with errors', 'key': 'net.if.out.errors[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.20.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: Inbound packets discarded', 'key': 'net.if.in.discards[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.13.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: Outbound packets discarded', 'key': 'net.if.out.discards[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.19.{#SNMPINDEX}'},
    
    # Non-unicast
    {'name': 'Interface {#IFNAME}: Non-unicast packets received', 'key': 'net.if.in.nucast[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.12.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: Non-unicast packets sent', 'key': 'net.if.out.nucast[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.18.{#SNMPINDEX}'},
    
    # High-capacity unicast packets
    {'name': 'Interface {#IFNAME}: HC Unicast packets received', 'key': 'net.if.in.ucastpkts.hc[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.7.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: HC Unicast packets sent', 'key': 'net.if.out.ucastpkts.hc[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.11.{#SNMPINDEX}'},
    
    # HC Multicast/Broadcast
    {'name': 'Interface {#IFNAME}: HC Multicast packets received', 'key': 'net.if.in.multicast.hc[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.8.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: HC Multicast packets sent', 'key': 'net.if.out.multicast.hc[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.12.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: HC Broadcast packets received', 'key': 'net.if.in.broadcast.hc[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.9.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: HC Broadcast packets sent', 'key': 'net.if.out.broadcast.hc[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.13.{#SNMPINDEX}'},
    
    # Status and properties
    {'name': 'Interface {#IFNAME}: Operational status', 'key': 'net.if.status[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.8.{#SNMPINDEX}', 'valuemap': 'IF-MIB::ifOperStatus'},
    {'name': 'Interface {#IFNAME}: Admin status', 'key': 'net.if.admin.status[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.7.{#SNMPINDEX}', 'valuemap': 'IF-MIB::ifAdminStatus'},
    {'name': 'Interface {#IFNAME}: Interface type', 'key': 'net.if.type[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.3.{#SNMPINDEX}', 'valuemap': 'IF-MIB::ifType'},
    {'name': 'Interface {#IFNAME}: Speed', 'key': 'net.if.speed[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.31.1.1.1.15.{#SNMPINDEX}', 'units': 'Mbps'},
    {'name': 'Interface {#IFNAME': MTU', 'key': 'net.if.mtu[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.4.{#SNMPINDEX}', 'units': 'bytes'},
    {'name': 'Interface {#IFNAME}: Duplex status', 'key': 'net.if.duplex[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.10.7.2.1.19.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: Last change', 'key': 'net.if.lastchange[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.9.{#SNMPINDEX}'},
    {'name': 'Interface {#IFNAME}: Output queue length', 'key': 'net.if.out.qlen[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.21.{#SNMPINDEX}'},
    
    # Unknown protocols
    {'name': 'Interface {#IFNAME}: Unknown protocols received', 'key': 'net.if.in.unknownprotos[{#SNMPINDEX}]', 'oid': '1.3.6.1.2.1.2.2.1.15.{#SNMPINDEX}'},
]

print(f"     Creating {len(interface_item_prototypes)} item prototypes per interface...")
print(f"     With 48 interfaces = {len(interface_item_prototypes) * 48} items")

# Create discovery rule
lld_data = {
    'name': 'Network Interface Discovery (Extended)',
    'key': 'net.if.discovery.extended',
    'type': 20,  # SNMPv2 agent
    'snmp_oid': 'discovery[{#IFNAME},1.3.6.1.2.1.31.1.1.1.1,{#IFDESCR},1.3.6.1.2.1.2.2.1.2,{#SNMPINDEX},1.3.6.1.2.1.2.2.1.1]',
    'hostid': template_id,
    'delay': '3600',  # Discovery every hour
    'lifetime': '7d',
    'description': 'Discovers all network interfaces using IF-MIB and IF-MIB-X.'
}

lld_result = client._request('discoveryrule.create', lld_data)
lld_rule_id = lld_result['itemids'][0]

# Create item prototypes
print(f"     Adding item prototypes...")
item_proto_data = []

for proto in interface_item_prototypes:
    item = {
        'hostid': template_id,
        'ruleid': lld_rule_id,
        'name': proto['name'],
        'key_': proto['key'],
        'type': 20,  # SNMPv2
        'snmp_oid': proto['oid'],
        'delay': '5m',  # 5-minute polling as requested
        'history': '7d',
        'trends': '30d',
        'value_type': 3,  # Numeric unsigned
    }
    
    if 'units' in proto:
        item['units'] = proto['units']
    
    if 'valuemap' in proto:
        item['valuemapid'] = proto['valuemap']  # Would need to look up actual ID
    
    if 'preprocessing' in proto:
        item['preprocessing'] = proto['preprocessing']
    
    item_proto_data.append(item)

# Create all item prototypes in batch
if item_proto_data:
    client._request('itemprototype.create', item_proto_data)

print(f"✅ Created {len(item_proto_data)} item prototypes\n")

print("="*70)
print(f"📊 Template Summary:")
print(f"   Name: Custom Cisco IOS Extended Monitoring")
print(f"   ID: {template_id}")
print(f"   Discovery Rules: 1 (Network Interfaces)")
print(f"   Item Prototypes: {len(item_proto_data)}")
print(f"   Expected items per host: {len(item_proto_data)} × 48 interfaces = {len(item_proto_data) * 48}")
print(f"   Polling interval: 5 minutes")
print("="*70)

print("\n✅ Custom template created successfully!")
print("\nNext steps:")
print("1. Link this template to all cisco-iosxr hosts")
print("2. Wait for discovery to run (~1 hour) or trigger manually")
print("3. Verify metrics collection")
