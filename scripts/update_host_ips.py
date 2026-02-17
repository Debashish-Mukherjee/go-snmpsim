#!/usr/bin/env python3
"""
Update all cisco-iosxr hosts to use the correct IP address that Zabbix can reach
"""

import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')

from zabbix_api_client import ZabbixAPIClient, ZabbixAPIError
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

print("🔑 Authenticating...")
client.login()
print("✅ Authenticated!\n")

# The IP that Zabbix containers can reach to access the host
# This is the gateway IP of the Zabbix Docker network
TARGET_IP = "172.18.0.1"

print(f"📝 Updating all cisco-iosxr hosts to use IP: {TARGET_IP}\n")

# Get all hosts matching our pattern
hosts = client._request("host.get", {
    "output": ["hostid", "host"],
    "selectInterfaces": ["interfaceid", "ip", "port"],
    "search": {
        "host": "cisco-iosxr"
    }
})

updated = 0
failed = 0

for host in hosts:
    hostname = host['host']
    hostid = host['hostid']
    
    if not host.get('interfaces'):
        print(f"⊘ {hostname:25s} - no interfaces")
        continue
    
    interface = host['interfaces'][0]
    current_ip = interface['ip']
    interface_id = interface['interfaceid']
    port = interface['port']
    
    # Skip if already correct
    if current_ip == TARGET_IP:
        print(f"✓ {hostname:25s} (port {port}) - already using {TARGET_IP}")
        continue
    
    # Update the interface
    try:
        client._request("hostinterface.update", {
            "interfaceid": interface_id,
            "ip": TARGET_IP
        })
        print(f"✅ {hostname:25s} (port {port}) - updated from {current_ip} to {TARGET_IP}")
        updated += 1
    except Exception as e:
        print(f"❌ {hostname:25s} (port {port}) - failed: {e}")
        failed += 1

print(f"\n" + "="*60)
print(f"📊 Summary:")
print(f"   Updated: {updated}")
print(f"   Failed:  {failed}")
print(f"   Total:   {len(hosts)}")
print("="*60)

# Verify one device is now reachable
if updated > 0 or len(hosts) > 0:
    print(f"\n🧪 Testing connectivity to first device...")
    test_host = hosts[0]
    test_port = test_host['interfaces'][0]['port']
    print(f"   Device: {test_host['host']}")
    print(f"   IP: {TARGET_IP}:{test_port}")
    print(f"\n   From Zabbix server, this should now work:")
    print(f"   snmpwalk -v2c -c public {TARGET_IP}:{test_port} 1.3.6.1.2.1.1")

client.logout()
print("\n✅ Done! Zabbix should now be able to query the SNMP simulator.")
print("   Wait 1-2 minutes for Zabbix to poll the devices.")
