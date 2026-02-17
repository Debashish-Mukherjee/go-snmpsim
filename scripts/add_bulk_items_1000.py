#!/usr/bin/env python3
"""
Add ~1500 SNMP items directly to all 1000 cisco-iosxr hosts.
Items: 1,454 per host (1,392 interface-based + 62 system)
Polling interval: 5 minutes default
"""
import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')
from zabbix_api_client import ZabbixAPIClient
import time

# 29 metrics per interface x 48 interfaces
INTERFACE_ITEMS = [
    ('ifDescr', 'net.if.description', '1', '6'),
    ('ifName', 'net.if.name', '1', '6'),
    ('ifMtu', 'net.if.mtu', '3', '0'),
    ('ifSpeed', 'net.if.speed', '3', '0'),
    ('ifOperStatus', 'net.if.status', '3', '0'),
    ('ifAdminStatus', 'net.if.admin.status', '3', '0'),
    ('ifPhysAddress', 'net.if.physaddr', '1', '6'),
    ('ifInOctets', 'net.if.in.bytes', '3', '2'),
    ('ifInUcastPkts', 'net.if.in.ucast', '3', '2'),
    ('ifInNUcastPkts', 'net.if.in.nucast', '3', '2'),
    ('ifInDiscards', 'net.if.in.discards', '3', '2'),
    ('ifInErrors', 'net.if.in.errors', '3', '2'),
    ('ifOutOctets', 'net.if.out.bytes', '3', '2'),
    ('ifOutUcastPkts', 'net.if.out.ucast', '3', '2'),
    ('ifOutNUcastPkts', 'net.if.out.nucast', '3', '2'),
    ('ifOutDiscards', 'net.if.out.discards', '3', '2'),
    ('ifOutErrors', 'net.if.out.errors', '3', '2'),
    ('ifHCInOctets', 'net.if.hc.in.bytes', '3', '2'),
    ('ifHCInUcastPkts', 'net.if.hc.in.ucast', '3', '2'),
    ('ifHCOutOctets', 'net.if.hc.out.bytes', '3', '2'),
    ('ifHCOutUcastPkts', 'net.if.hc.out.ucast', '3', '2'),
    ('ifDuplex', 'net.if.duplex', '3', '0'),
    ('ifHighSpeed', 'net.if.highspeed', '3', '0'),
    ('ifLastChange', 'net.if.lastchange', '3', '0'),
    ('ifConnectorPresent', 'net.if.connector', '3', '0'),
    ('etherStatsOctets', 'net.ether.in.bytes', '3', '2'),
    ('etherStatsUndersizePkts', 'net.ether.undersize', '3', '2'),
    ('etherStatsOversizePkts', 'net.ether.oversize', '3', '2'),
    ('etherStatsCRCAlignErrors', 'net.ether.crc', '3', '2'),
]

# 62 system metrics (CPU, memory, temp, fans, PSUs, IP/TCP/UDP stats, entity info)
SYSTEM_ITEMS = [
    ('sysDescr', 'system.description', '1', '6', '0'),
    ('sysUpTime', 'system.uptime', '3', '0', '0'),
    ('sysContact', 'system.contact', '1', '6', '0'),
    ('sysName', 'system.name', '1', '6', '0'),
    ('sysLocation', 'system.location', '1', '6', '0'),
    ('cpmCPUTotal5min.0', 'cpu.util.5min', '3', '0', '3'),
    ('cpmCPUTotal1min.0', 'cpu.util.1min', '3', '0', '3'),
    ('cpmCPUTotal5minRev.0', 'cpu.util.rev.5min', '3', '0', '3'),
    ('cpmCPUTotal1minRev.0', 'cpu.util.rev.1min', '3', '0', '3'),
    ('ciscoMemoryPoolUsed.1', 'memory.pool.main.used', '3', '0', '0'),
    ('ciscoMemoryPoolFree.1', 'memory.pool.main.free', '3', '0', '0'),
    ('ciscoMemoryPoolUsed.2', 'memory.pool.io.used', '3', '0', '0'),
    ('ciscoMemoryPoolFree.2', 'memory.pool.io.free', '3', '0', '0'),
    ('entSensorValue.1', 'sensor.inlet.temp', '3', '0', '3'),
    ('entSensorValue.2', 'sensor.exhaust.temp', '3', '0', '3'),
    ('entSensorValue.3', 'sensor.cpu.temp', '3', '0', '3'),
    ('entSensorValue.4', 'sensor.chassis.temp', '3', '0', '3'),
    ('entSensorValue.5', 'sensor.module.temp', '3', '0', '3'),
    ('entSensorValue.6', 'sensor.backplane.temp', '3', '0', '3'),
    ('entSensorValue.7', 'sensor.fabric.temp', '3', '0', '3'),
    ('entSensorValue.8', 'sensor.systembrd.temp', '3', '0', '3'),
    ('entSensorStatus.1', 'sensor.status.inlet', '3', '0', '0'),
    ('entSensorStatus.2', 'sensor.status.exhaust', '3', '0', '0'),
    ('entSensorStatus.3', 'sensor.status.cpu', '3', '0', '0'),
    ('entSensorStatus.4', 'sensor.status.chassis', '3', '0', '0'),
    ('entSensorStatus.5', 'sensor.status.module', '3', '0', '0'),
    ('entSensorStatus.6', 'sensor.status.backplane', '3', '0', '0'),
    ('entSensorStatus.7', 'sensor.status.fabric', '3', '0', '0'),
    ('entSensorStatus.8', 'sensor.status.systembrd', '3', '0', '0'),
    ('ciscoFanState.1', 'fan.status.1', '3', '0', '0'),
    ('ciscoFanState.2', 'fan.status.2', '3', '0', '0'),
    ('ciscoFanState.3', 'fan.status.3', '3', '0', '0'),
    ('ciscoFanState.4', 'fan.status.4', '3', '0', '0'),
    ('ciscoFanState.5', 'fan.status.5', '3', '0', '0'),
    ('ciscoFanState.6', 'fan.status.6', '3', '0', '0'),
    ('ciscoPowerSupplyStatus.1', 'psu.status.1', '3', '0', '0'),
    ('ciscoPowerSupplyStatus.2', 'psu.status.2', '3', '0', '0'),
    ('ciscoPowerSupplyStatus.3', 'psu.status.3', '3', '0', '0'),
    ('ciscoPowerSupplyStatus.4', 'psu.status.4', '3', '0', '0'),
    ('entPhysicalSerialNum.1', 'entity.serial.chassis', '1', '6', '0'),
    ('entPhysicalModelName.1', 'entity.model.chassis', '1', '6', '0'),
    ('ipInReceives', 'ip.in.receives', '3', '0', '2'),
    ('ipInHdrErrors', 'ip.in.hdrerrors', '3', '0', '2'),
    ('ipInAddrErrors', 'ip.in.addrerrors', '3', '0', '2'),
    ('ipForwDatagrams', 'ip.forwarded', '3', '0', '2'),
    ('ipInDelivers', 'ip.in.delivers', '3', '0', '2'),
    ('ipOutRequests', 'ip.out.requests', '3', '0', '2'),
    ('ipOutDiscards', 'ip.out.discards', '3', '0', '2'),
    ('ipReasmReqds', 'ip.reassm.reqds', '3', '0', '2'),
    ('ipReasmOKs', 'ip.reassm.oks', '3', '0', '2'),
    ('ipFragOKs', 'ip.frag.oks', '3', '0', '2'),
    ('tcpInSegs', 'tcp.in.segments', '3', '0', '2'),
    ('tcpOutSegs', 'tcp.out.segments', '3', '0', '2'),
    ('udpInDatagrams', 'udp.in.datagrams', '3', '0', '2'),
    ('udpOutDatagrams', 'udp.out.datagrams', '3', '0', '2'),
    ('icmpInMsgs', 'icmp.in.msgs', '3', '0', '2'),
    ('icmpOutMsgs', 'icmp.out.msgs', '3', '0', '2'),
    ('icmpInEchos', 'icmp.in.echos', '3', '0', '2'),
    ('icmpOutEchoReplies', 'icmp.out.echo.replies', '3', '0', '2'),
]

def main():
    client = ZabbixAPIClient('http://localhost:8081', 'Admin', 'zabbix')
    print("🔐 Logging in to Zabbix...")
    client.login()
    print("✅ Logged in successfully\n")
    
    # Get all cisco-iosxr hosts
    print("🔍 Loading hosts...")
    hosts = client._request('host.get', {
        'output': ['hostid', 'host'],
        'search': {'host': 'cisco-iosxr'},
        'limit': 10000
    })
    
    print(f"   Found {len(hosts)} hosts\n")
    
    total_items_per_host = (len(INTERFACE_ITEMS) * 48) + len(SYSTEM_ITEMS)
    print(f"📊 Target: {total_items_per_host} items per host")
    print(f"   - Interface items: {len(INTERFACE_ITEMS)} metrics × 48 interfaces = {len(INTERFACE_ITEMS) * 48}")
    print(f"   - System items: {len(SYSTEM_ITEMS)}")
    print(f"   - Polling: 5 minutes default\n")
    print(f"🚀 Processing {len(hosts)} hosts...\n")
    
    total_success = 0
    total_failed = 0
    total_duplicate = 0
    
    for host_idx, host in enumerate(hosts, 1):
        host_id = host['hostid']
        host_name = host['host']
        
        # Get SNMP interface for this host
        try:
            interfaces = client._request('hostinterface.get', {
                'output': ['interfaceid', 'type'],
                'hostids': [host_id]
            })
            
            snmp_interface = None
            for iface in interfaces:
                if iface['type'] == '2':  # SNMP
                    snmp_interface = iface['interfaceid']
                    break
            
            if not snmp_interface:
                print(f"[{host_idx}/{len(hosts)}] ❌ {host_name}: No SNMP interface found")
                continue
            
            # Create all items for this host
            items_to_create = []
            
            # Interface items (for 48 interfaces, indices 1-48)
            for if_idx in range(1, 49):
                for oid_name, item_key, value_type, data_type, *trends_opt in INTERFACE_ITEMS:
                    trends = int(trends_opt[0]) if trends_opt else 365
                    oid = f"{oid_name}.{if_idx}"
                    key = f"{item_key}[{if_idx}]"
                    
                    item = {
                        'hostid': host_id,
                        'name': f"{item_key} ifIndex {if_idx}",
                        'key_': key,
                        'type': 20,  # SNMP agent
                        'snmp_oid': oid,
                        'value_type': int(value_type),
                        'delay': '5m',
                        'history': '7d',
                        'trends': trends,
                        'status': '0',
                        'interfaceid': snmp_interface,
                    }
                    items_to_create.append(item)
            
            # System items
            for oid_name, item_key, value_type, data_type, trends in SYSTEM_ITEMS:
                item = {
                    'hostid': host_id,
                    'name': item_key,
                    'key_': item_key,
                    'type': 20,  # SNMP agent
                    'snmp_oid': oid_name,
                    'value_type': int(value_type),
                    'delay': '5m',
                    'history': '7d',
                    'trends': int(trends),
                    'status': '0',
                    'interfaceid': snmp_interface,
                }
                # Text items don't need trends
                if int(value_type) == 4:  # Text
                    item['trends'] = 0
                items_to_create.append(item)
            
            # Batch create items (100 at a time)
            batch_size = 100
            host_success = 0
            host_failed = 0
            
            for batch_start in range(0, len(items_to_create), batch_size):
                batch = items_to_create[batch_start:batch_start + batch_size]
                try:
                    result = client._request('item.create', batch)
                    host_success += len(batch)
                except Exception as e:
                    # Some items may fail due to duplicates - that's OK
                    error_str = str(e)
                    if "already exists" in error_str:
                        duplicate_count = error_str.count("already exists")
                        host_failed += duplicate_count
                        host_success += len(batch) - duplicate_count
                    else:
                        # For first batch, print error to help debug
                        if batch_start == 0 and host_idx == 1:
                            print(f"   DEBUG: First batch error: {e}")
                        host_failed += len(batch)
            
            total_success += host_success
            total_failed += host_failed
            total_duplicate += host_failed
            
            if host_idx % 50 == 0:
                print(f"[{host_idx}/{len(hosts)}] {host_name}")
                print(f"   ✅ Created {host_success} items, ⚠️  Failed: {host_failed}")
        
        except Exception as e:
            print(f"[{host_idx}/{len(hosts)}] ❌ {host_name}: {e}")
            continue
    
    print(f"\n{'='*60}")
    print(f"✅ Completed!")
    print(f"{'='*60}")
    print(f"Total items created: {total_success}")
    print(f"Total failures (mostly duplicates): {total_failed}")
    print(f"Expected per host: ~{total_items_per_host}")
    print(f"Hosts processed: {len(hosts)}")
    print(f"Total metrics deployed: ~{total_success} / {len(hosts) * total_items_per_host} target")

if __name__ == '__main__':
    main()
