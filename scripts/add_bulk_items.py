#!/usr/bin/env python3
"""
Add comprehensive SNMP items directly to all cisco-iosxr hosts.
Target: ~1500 items per host with 5-minute polling.

Strategy: 
- 48 interfaces × 30 metrics = 1,440 interface items
- ~60 system items (CPU, memory, temp, fan, PSU, etc.)
- Total: ~1,500 items per host
"""

import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')
from zabbix_api_client import ZabbixAPIClient
import yaml
import time

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

# Define comprehensive interface items (30 metrics per interface)
INTERFACE_ITEMS = [
    # Traffic counters (HC = High Capacity 64-bit)
    {'name': 'Interface {ifindex}: Bits received', 'key': 'net.if.in.bits[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.6.{ifindex}', 'units': 'bps', 'value_type': 3},
    {'name': 'Interface {ifindex}: Bits sent', 'key': 'net.if.out.bits[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.10.{ifindex}', 'units': 'bps', 'value_type': 3},
    
    # Packet counters
    {'name': 'Interface {ifindex}: Packets received', 'key': 'net.if.in.packets[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.11.{ifindex}', 'units': 'pps', 'value_type': 3},
    {'name': 'Interface {ifindex}: Packets sent', 'key': 'net.if.out.packets[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.17.{ifindex}', 'units': 'pps', 'value_type': 3},
    
    # Unicast packets (HC)
    {'name': 'Interface {ifindex}: HC Unicast packets in', 'key': 'net.if.in.ucast.hc[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.7.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: HC Unicast packets out', 'key': 'net.if.out.ucast.hc[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.11.{ifindex}', 'value_type': 3},
    
    # Multicast packets
    {'name': 'Interface {ifindex}: Multicast packets in', 'key': 'net.if.in.mcast[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.2.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Multicast packets out', 'key': 'net.if.out.mcast[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.4.{ifindex}', 'value_type': 3},
    
    # HC Multicast (64-bit)
    {'name': 'Interface {ifindex}: HC Multicast packets in', 'key': 'net.if.in.mcast.hc[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.8.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: HC Multicast packets out', 'key': 'net.if.out.mcast.hc[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.12.{ifindex}', 'value_type': 3},
    
    # Broadcast packets
    {'name': 'Interface {ifindex}: Broadcast packets in', 'key': 'net.if.in.bcast[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.3.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Broadcast packets out', 'key': 'net.if.out.bcast[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.5.{ifindex}', 'value_type': 3},
    
    # HC Broadcast (64-bit)
    {'name': 'Interface {ifindex}: HC Broadcast packets in', 'key': 'net.if.in.bcast.hc[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.9.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: HC Broadcast packets out', 'key': 'net.if.out.bcast.hc[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.13.{ifindex}', 'value_type': 3},
    
    # Non-unicast packets
    {'name': 'Interface {ifindex}: Non-unicast packets in', 'key': 'net.if.in.nucast[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.12.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Non-unicast packets out', 'key': 'net.if.out.nucast[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.18.{ifindex}', 'value_type': 3},
    
    # Errors and discards
    {'name': 'Interface {ifindex}: Inbound errors', 'key': 'net.if.in.errors[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.14.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Outbound errors', 'key': 'net.if.out.errors[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.20.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Inbound discards', 'key': 'net.if.in.discards[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.13.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Outbound discards', 'key': 'net.if.out.discards[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.19.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Unknown protocols', 'key': 'net.if.in.unknownprotos[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.15.{ifindex}', 'value_type': 3},
    
    # Status and properties
    {'name': 'Interface {ifindex}: Operational status', 'key': 'net.if.operstatus[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.8.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Admin status', 'key': 'net.if.adminstatus[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.7.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Speed Mbps', 'key': 'net.if.speed[{ifindex}]', 'oid': '1.3.6.1.2.1.31.1.1.1.15.{ifindex}', 'units': 'Mbps', 'value_type': 3},
    {'name': 'Interface {ifindex}: MTU', 'key': 'net.if.mtu[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.4.{ifindex}', 'units': 'bytes', 'value_type': 3},
    {'name': 'Interface {ifindex}: Type', 'key': 'net.if.type[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.3.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Duplex', 'key': 'net.if.duplex[{ifindex}]', 'oid': '1.3.6.1.2.1.10.7.2.1.19.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Last change', 'key': 'net.if.lastchange[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.9.{ifindex}', 'value_type': 3},
    {'name': 'Interface {ifindex}: Output queue length', 'key': 'net.if.outqlen[{ifindex}]', 'oid': '1.3.6.1.2.1.2.2.1.21.{ifindex}', 'value_type': 3},
]

# System items (beyond interfaces)
SYSTEM_ITEMS = [
    # System info
    {'name': 'System: Description', 'key': 'system.descr', 'oid': '1.3.6.1.2.1.1.1.0', 'value_type': 4},
    {'name': 'System: Uptime', 'key': 'system.uptime', 'oid': '1.3.6.1.2.1.1.3.0', 'value_type': 3},
    {'name': 'System: Contact', 'key': 'system.contact', 'oid': '1.3.6.1.2.1.1.4.0', 'value_type': 4},
    {'name': 'System: Name', 'key': 'system.name', 'oid': '1.3.6.1.2.1.1.5.0', 'value_type': 4},
    {'name': 'System: Location', 'key': 'system.location', 'oid': '1.3.6.1.2.1.1.6.0', 'value_type': 4},
    
    # IP stats
    {'name': 'IP: Packets received', 'key': 'ip.in.receives', 'oid': '1.3.6.1.2.1.4.3.0', 'value_type': 3},
    {'name': 'IP: Header errors', 'key': 'ip.in.hdrerrors', 'oid': '1.3.6.1.2.1.4.4.0', 'value_type': 3},
    {'name': 'IP: Address errors', 'key': 'ip.in.addrerrors', 'oid': '1.3.6.1.2.1.4.5.0', 'value_type': 3},
    {'name': 'IP: Forwarded datagrams', 'key': 'ip.forw.datagrams', 'oid': '1.3.6.1.2.1.4.6.0', 'value_type': 3},
    {'name': 'IP: Discarded packets', 'key': 'ip.in.discards', 'oid': '1.3.6.1.2.1.4.8.0', 'value_type': 3},
    {'name': 'IP: Delivered packets', 'key': 'ip.in.delivers', 'oid': '1.3.6.1.2.1.4.9.0', 'value_type': 3},
    {'name': 'IP: Output requests', 'key': 'ip.out.requests', 'oid': '1.3.6.1.2.1.4.10.0', 'value_type': 3},
    
    # TCP stats
    {'name': 'TCP: Active opens', 'key': 'tcp.active.opens', 'oid': '1.3.6.1.2.1.6.5.0', 'value_type': 3},
    {'name': 'TCP: Passive opens', 'key': 'tcp.passive.opens', 'oid': '1.3.6.1.2.1.6.6.0', 'value_type': 3},
    {'name': 'TCP: Failed attempts', 'key': 'tcp.attempt.fails', 'oid': '1.3.6.1.2.1.6.7.0', 'value_type': 3},
    {'name': 'TCP: Established resets', 'key': 'tcp.estab.resets', 'oid': '1.3.6.1.2.1.6.8.0', 'value_type': 3},
    {'name': 'TCP: Current established', 'key': 'tcp.curr.estab', 'oid': '1.3.6.1.2.1.6.9.0', 'value_type': 3},
    {'name': 'TCP: Segments received', 'key': 'tcp.in.segs', 'oid': '1.3.6.1.2.1.6.10.0', 'value_type': 3},
    {'name': 'TCP: Segments sent', 'key': 'tcp.out.segs', 'oid': '1.3.6.1.2.1.6.11.0', 'value_type': 3},
    {'name': 'TCP: Retransmitted segments', 'key': 'tcp.retrans.segs', 'oid': '1.3.6.1.2.1.6.12.0', 'value_type': 3},
    
    # UDP stats
    {'name': 'UDP: Datagrams received', 'key': 'udp.in.datagrams', 'oid': '1.3.6.1.2.1.7.1.0', 'value_type': 3},
    {'name': 'UDP: No ports', 'key': 'udp.no.ports', 'oid': '1.3.6.1.2.1.7.2.0', 'value_type': 3},
    {'name': 'UDP: Input errors', 'key': 'udp.in.errors', 'oid': '1.3.6.1.2.1.7.3.0', 'value_type': 3},
    {'name': 'UDP: Datagrams sent', 'key': 'udp.out.datagrams', 'oid': '1.3.6.1.2.1.7.4.0', 'value_type': 3},
]

# CPU items (4 CPUs)
for cpu_idx in range(1, 5):
    SYSTEM_ITEMS.extend([
        {'name': f'CPU {cpu_idx}: 1min utilization', 'key': f'cpu.util.1min[{cpu_idx}]', 'oid': f'1.3.6.1.4.1.9.9.109.1.1.1.1.3.{cpu_idx}', 'units': '%', 'value_type': 3},
        {'name': f'CPU {cpu_idx}: 5min utilization', 'key': f'cpu.util.5min[{cpu_idx}]', 'oid': f'1.3.6.1.4.1.9.9.109.1.1.1.1.4.{cpu_idx}', 'units': '%', 'value_type': 3},
    ])

# Memory items (2 pools)
for mem_idx in range(1, 3):
    SYSTEM_ITEMS.extend([
        {'name': f'Memory Pool {mem_idx}: Used', 'key': f'memory.used[{mem_idx}]', 'oid': f'1.3.6.1.4.1.9.9.48.1.1.1.5.{mem_idx}', 'units': 'B', 'value_type': 3},
        {'name': f'Memory Pool {mem_idx}: Free', 'key': f'memory.free[{mem_idx}]', 'oid': f'1.3.6.1.4.1.9.9.48.1.1.1.6.{mem_idx}', 'units': 'B', 'value_type': 3},
    ])

# Temperature sensors (8 sensors)
for sensor_idx in range(1, 9):
    SYSTEM_ITEMS.extend([
        {'name': f'Temperature Sensor {sensor_idx}: Value', 'key': f'temp.value[{sensor_idx}]', 'oid': f'1.3.6.1.4.1.9.9.13.1.3.1.3.{sensor_idx}', 'units': '°C', 'value_type': 3},
        {'name': f'Temperature Sensor {sensor_idx}: State', 'key': f'temp.state[{sensor_idx}]', 'oid': f'1.3.6.1.4.1.9.9.13.1.3.1.4.{sensor_idx}', 'value_type': 3},
    ])

# Fan sensors (6 fans)
for fan_idx in range(1, 7):
    SYSTEM_ITEMS.append({
        'name': f'Fan {fan_idx}: State', 'key': f'fan.state[{fan_idx}]', 'oid': f'1.3.6.1.4.1.9.9.13.1.4.1.3.{fan_idx}', 'value_type': 3
    })

# Power supplies (4 PSUs)
for psu_idx in range(1, 5):
    SYSTEM_ITEMS.append({
        'name': f'PSU {psu_idx}: State', 'key': f'psu.state[{psu_idx}]', 'oid': f'1.3.6.1.4.1.9.9.13.1.5.1.3.{psu_idx}', 'value_type': 3
    })

print(f"📊 Item Configuration:")
print(f"   Interface items per port: {len(INTERFACE_ITEMS)}")
print(f"   System items: {len(SYSTEM_ITEMS)}")
print(f"   Total for 48 ports: {48 * len(INTERFACE_ITEMS) + len(SYSTEM_ITEMS)}")
print()

# Get all cisco-iosxr hosts
print("🔍 Loading hosts...")
hosts = client._request('host.get', {
    'output': ['hostid', 'host'],
    'search': {'host': 'cisco-iosxr'},
    'limit': 10000  # Ensure we get all hosts (up to 10000)
})

print(f"   Found {len(hosts)} hosts\n")

# Apply to first host only (test)
TEST_MODE = False
if TEST_MODE:
    print("⚠️  TEST MODE: Applying to first host only")
    print("   If successful, disable TEST_MODE and run again for all hosts\n")
    hosts = hosts[:1]
else:
    print(f"✅ PRODUCTION MODE: Applying to all {len(hosts)} hosts\n")

for host_idx, host in enumerate(hosts, 1):
    print(f"[{host_idx}/{len(hosts)}] Processing {host['host']}...")
    
    # Get host's SNMP interface ID
    interfaces = client._request('hostinterface.get', {
        'output': ['interfaceid', 'type'],
        'hostids': [host['hostid']]
    })
    
    snmp_interface = None
    for iface in interfaces:
        if iface['type'] == '2':  # Type 2 = SNMP
            snmp_interface = iface['interfaceid']
            break
    
    if not snmp_interface:
        print(f"   ❌ No SNMP interface found, skipping...")
        continue
    
    print(f"   SNMP Interface ID: {snmp_interface}")
    
    items_to_create = []
    
    # Add interface items for all 48 ports
    for ifindex in range(1, 49):
        for item_template in INTERFACE_ITEMS:
            item = {
                'hostid': host['hostid'],
                'interfaceid': snmp_interface,
                'name': item_template['name'].replace('{ifindex}', str(ifindex)),
                'key_': item_template['key'].replace('{ifindex}', str(ifindex)),
                'type': 20,  # SNMPv2
                'snmp_oid': item_template['oid'].replace('{ifindex}', str(ifindex)),
                'delay': '5m',  # 5-minute polling
                'history': '7d',
                'trends': '365d',
                'value_type': item_template['value_type']
            }
            
            if 'units' in item_template:
                item['units'] = item_template['units']
            
            items_to_create.append(item)
    
    # Add system items
    for item_template in SYSTEM_ITEMS:
        item = {
            'hostid': host['hostid'],
            'interfaceid': snmp_interface,
            'name': item_template['name'],
            'key_': item_template['key'],
            'type': 20,  # SNMPv2
            'snmp_oid': item_template['oid'],
            'delay': '5m',  # 5-minute polling
            'history': '7d',
            'trends': '0' if item_template['value_type'] == 4 else '365d',  # No trends for text items
            'value_type': item_template['value_type']
        }
        
        if 'units' in item_template:
            item['units'] = item_template['units']
        
        items_to_create.append(item)
    
    print(f"   Creating {len(items_to_create)} items...")
    
    # Create items in batches of 100 to avoid API limits
    BATCH_SIZE = 100
    created = 0
    failed = 0
    
    for i in range(0, len(items_to_create), BATCH_SIZE):
        batch = items_to_create[i:i+BATCH_SIZE]
        try:
            client._request('item.create', batch)
            created += len(batch)
            print(f"   Progress: {created}/{len(items_to_create)}...", end='\r')
        except Exception as e:
            print(f"\n   ❌ Batch {i//BATCH_SIZE + 1} failed: {e}")
            failed += len(batch)
    
    print(f"\n   ✅ Created {created} items, Failed: {failed}")
    print()

print("="*70)
print(f"✅ Completed!")
print(f"\n📈 Summary:")
print(f"   Hosts processed: {len(hosts)}")
print(f"   Items per host: {48 * len(INTERFACE_ITEMS) + len(SYSTEM_ITEMS)}")
print(f"   Polling interval: 5 minutes")
print(f"\n{'⚠️  TEST MODE - Only 1 host updated' if TEST_MODE else '✅ All hosts updated'}")
if TEST_MODE:
    print(f"\n   Next: Edit script, set TEST_MODE = False, run again for all 100 hosts")
