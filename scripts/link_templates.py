#!/usr/bin/env python3
"""
Link Cisco IOS SNMP template to all cisco-iosxr hosts
"""

import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')
from zabbix_api_client import ZabbixAPIClient
import yaml

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

# Get Cisco IOS template
print("📦 Finding Cisco IOS SNMP template...")
templates = client._request('template.get', {
    'output': ['templateid', 'host', 'name'],
    'filter': {'host': ['Cisco IOS by SNMP']}
})

if not templates:
    print("❌ Cisco IOS by SNMP template not found!")
    exit(1)

template = templates[0]
template_id = template['templateid']
print(f"   Found: {template['name']} (ID: {template_id})\n")

# Get all cisco-iosxr hosts
print("🔍 Finding cisco-iosxr hosts...")
hosts = client._request('host.get', {
    'output': ['hostid', 'host'],
    'search': {'host': 'cisco-iosxr'}
})

print(f"   Found {len(hosts)} hosts\n")

# Link template to hosts
print("🔗 Linking template to hosts...")
success_count = 0
failed_count = 0

for host in hosts:
    try:
        # Link template using host.update
        client._request('host.update', {
            'hostid': host['hostid'],
            'templates': [{'templateid': template_id}]
        })
        print(f"   ✅ {host['host']}")
        success_count += 1
    except Exception as e:
        print(f"   ❌ {host['host']}: {e}")
        failed_count += 1

print("\n" + "="*60)
print(f"📊 Summary:")
print(f"   Linked:  {success_count}")
print(f"   Failed:  {failed_count}")
print(f"   Total:   {len(hosts)}")
print("="*60)

# Verify items were added
print("\n🔍 Verifying items for first host...")
if hosts:
    items = client._request('item.get', {
        'output': ['name'],
        'hostids': [hosts[0]['hostid']]
    })
    print(f"   Host: {hosts[0]['host']}")
    print(f"   Items: {len(items)} configured")
    
    if len(items) > 0:
        print("\n✅ Success! Hosts now have monitoring items configured.")
        print("   Zabbix will start collecting data within 1-2 minutes.")
    else:
        print("\n⚠️  Template linked but no items found. May need to wait a moment...")
