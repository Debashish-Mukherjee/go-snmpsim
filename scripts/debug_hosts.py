#!/usr/bin/env python3
"""
Debug - check what hosts exist
"""

import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')

from zabbix_api_client import ZabbixAPIClient
import yaml

# Load config
with open('/home/debashish/trials/go-snmpsim/zabbix/zabbix_config.yaml', 'r') as f:
    config = yaml.safe_load(f)

client = ZabbixAPIClient(config['zabbix_url'], config['zabbix_username'], config['zabbix_password'])
client.login()

# Get ALL hosts
print("Getting all hosts...")
hosts = client._request("host.get", {
    "output": ["hostid", "host", "name"],
    "selectInterfaces": ["interfaceid", "ip", "port", "type"],
    "limit": 10
})

print(f"\nFound {len(hosts)} hosts:")
for host in hosts:
    print(f"\n  Host: {host['host']}")
    print(f"    ID: {host['hostid']}")
    if host.get('interfaces'):
        for iface in host['interfaces']:
            print(f"    Interface: {iface['ip']}:{iface['port']} (type {iface['type']})")

# Try to get hosts with search
print("\n\nSearching for cisco-iosxr hosts...")
hosts2 = client._request("host.get", {
    "output": ["hostid", "host"],
    "selectInterfaces": ["interfaceid", "ip", "port"],
    "search": {
        "host": "cisco-iosxr"
    },
    "limit": 5
})

print(f"Found {len(hosts2)} hosts with search:")
for host in hosts2:
    print(f"  - {host['host']}")
    if host.get('interfaces'):
        print(f"    IP: {host['interfaces'][0]['ip']}:{host['interfaces'][0]['port']}")

client.logout()
