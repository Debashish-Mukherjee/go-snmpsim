#!/usr/bin/env python3
"""
Add cisco-iosxr-101 through cisco-iosxr-1000 to Zabbix (900 new hosts)
"""
import sys
sys.path.insert(0, '/home/debashish/trials/go-snmpsim/zabbix')
from zabbix_api_client import ZabbixAPIClient
import time

def main():
    client = ZabbixAPIClient('http://localhost:8081', 'Admin', 'zabbix')
    print("Logging in to Zabbix...")
    client.login()
    print("✅ Logged in successfully")
    
    # Get the Cisco IOS template ID
    templates = client._request('template.get', {
        'output': ['templateid', 'name'],
        'filter': {'name': 'Cisco IOS by SNMP'}
    })
    
    if not templates:
        print("❌ Cisco IOS by SNMP template not found")
        return
    
    template_id = templates[0]['templateid']
    print(f"✅ Found template ID: {template_id}")
    
    # Get existing groups  
    groups = client._request('hostgroup.get', {
        'output': ['groupid', 'name'],
        'filter': {'name': ['Linux servers', 'Zabbix servers']}
    })
    
    group_ids = [{'groupid': g['groupid']} for g in groups]
    print(f"✅ Using groups: {[g['name'] for g in groups]}")
    
    # Add hosts 101-1000
    start_host = 101
    end_host = 1000
    total_hosts = end_host - start_host + 1
    
    print(f"\n🚀 Adding {total_hosts} hosts (cisco-iosxr-{start_host:03d} to cisco-iosxr-{end_host:03d})...")
    
    added = 0
    failed = 0
    batch_size = 10
    
    for i in range(start_host, end_host + 1):
        host_name = f"cisco-iosxr-{i:03d}"
        port = 20000 + (i - 1)  # Port mapping: host-001 -> 20000, host-002 -> 20001, etc.
        
        try:
            result = client._request('host.create', {
                'host': host_name,
                'name': f'Cisco IOS XR Router {i:03d}',
                'interfaces': [{
                    'type': 2,  # SNMP
                    'main': 1,
                    'useip': 1,
                    'ip': '172.18.0.1',  # Docker gateway IP
                    'dns': '',
                    'port': str(port),
                    'details': {
                        'version': 3,
                        'bulk': 1,
                        'securityname': 'simuser',
                        'securitylevel': 0,
                        'contextname': '',
                        'max_repetitions': 10
                    }
                }],
                'groups': group_ids,
                'templates': [{'templateid': template_id}]
            })
            
            added += 1
            if added % batch_size == 0:
                print(f"   [{added}/{total_hosts}] Created {host_name} on port {port}")
            
        except Exception as e:
            failed += 1
            if "already exists" not in str(e):
                print(f"   ❌ Failed {host_name}: {e}")
        
        # Rate limiting - don't overwhelm the API
        if i % 50 == 0:
            time.sleep(1)
    
    print(f"\n✅ Completed!")
    print(f"   Added: {added}")
    print(f"   Failed: {failed}")
    print(f"   Total Zabbix hosts: {added + 100} (assuming original 100 still exist)")

if __name__ == '__main__':
    main()
