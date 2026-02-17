#!/usr/bin/env python3
"""Check detailed configuration of a Zabbix host."""

from zabbix.zabbix_api_client import ZabbixAPIClient

# Connect to Zabbix
print("🔗 Connecting to Zabbix...")
client = ZabbixAPIClient("http://localhost:8081", "Admin", "zabbix")
print("✅ Authenticated!")
print()

# Get detailed info for one host
hosts = client._request("host.get", {
    "output": ["hostid", "host", "status", "available", "snmp_available", "error", "snmp_error"],
    "selectInterfaces": ["interfaceid", "ip", "port", "type", "useip"],
    "search": {
        "host": "cisco-iosxr-001"
    }
})

# Get items separately
items = []
if hosts:
    items = client._request("item.get", {
        "output": ["itemid", "name", "key_", "type", "delay", "status"],
        "hostids": [hosts[0]["hostid"]],
        "limit": 10
    })

if not hosts:
    print("❌ No host found!")
    exit(1)

host = hosts[0]

print(f"🔍 Host: {host['host']}")
print(f"   ID: {host['hostid']}")
print(f"   Status: {'✅ Enabled' if host['status'] == '0' else '❌ Disabled'}")
print(f"   Available: {host.get('available', 'unknown')} (1=yes, 2=no, 0=unknown)")
print(f"   SNMP Available: {host.get('snmp_available', 'unknown')} (1=yes, 2=no, 0=unknown)")
if host.get('error'):
    print(f"   Error: {host['error']}")
if host.get('snmp_error'):
    print(f"   SNMP Error: {host['snmp_error']}")
print()

print("📡 Interfaces:")
for iface in host.get('interfaces', []):
    type_name = {1: "Agent", 2: "SNMP", 3: "IPMI", 4: "JMX"}.get(int(iface['type']), "Unknown")
    print(f"   [{type_name}] {iface['ip']}:{iface['port']} (useip={iface.get('useip', '?')})")
print()

print(f"📊 Items: {len(items)} configured")
if items:
    print("   First 5 items:")
    for item in items[:5]:
        status = "✅" if item['status'] == '0' else "❌"
        print(f"   {status} {item['name']} ({item['key_']})")
        print(f"      Type: {item['type']}, Delay: {item['delay']}")
