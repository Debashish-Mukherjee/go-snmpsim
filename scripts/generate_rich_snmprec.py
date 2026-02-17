#!/usr/bin/env python3
"""
Generate a rich SNMPREC file with 48 interfaces and comprehensive metrics.
Target: ~1500 metrics per device for Zabbix testing.
"""

import random

def generate_snmprec():
    """Generate comprehensive SNMP data file"""
    
    output = []
    
    # Header
    output.append("# SNMP Simulator Rich Data File")
    output.append("# Generated for high-volume metric testing")
    output.append("# Format: OID|TYPE|VALUE")
    output.append("")
    
    # =======================================================
    # SYSTEM GROUP (1.3.6.1.2.1.1)
    # =======================================================
    output.append("# ============================================================")
    output.append("# SYSTEM GROUP (1.3.6.1.2.1.1)")
    output.append("# ============================================================")
    output.append("")
    
    output.append("1.3.6.1.2.1.1.1.0|octetstring|Cisco IOS Software, 7200 Software (C7200-ADVIPSERVICESK9-M), Version 15.2(4)S7")
    output.append("1.3.6.1.2.1.1.2.0|objectidentifier|1.3.6.1.4.1.9.1.222")
    output.append("1.3.6.1.2.1.1.3.0|timeticks|1234567890")
    output.append("1.3.6.1.2.1.1.4.0|octetstring|Network Operations <netops@company.com>")
    output.append("1.3.6.1.2.1.1.5.0|octetstring|cisco-core-switch")
    output.append("1.3.6.1.2.1.1.6.0|octetstring|Data Center DC1 - Rack 15 - Row B")
    output.append("1.3.6.1.2.1.1.7.0|integer|78")
    output.append("")
    
    # =======================================================
    # INTERFACES (48 ports with full metrics)
    # =======================================================
    output.append("# ============================================================")
    output.append("# INTERFACES GROUP (1.3.6.1.2.1.2)")
    output.append("# 48 GigabitEthernet interfaces with comprehensive metrics")
    output.append("# ============================================================")
    output.append("")
    
    NUM_INTERFACES = 48
    output.append(f"1.3.6.1.2.1.2.1.0|integer|{NUM_INTERFACES}")
    output.append("")
    
    for ifIndex in range(1, NUM_INTERFACES + 1):
        output.append(f"# Interface {ifIndex}: GigabitEthernet1/0/{ifIndex}")
        
        # Basic interface info
        output.append(f"1.3.6.1.2.1.2.2.1.1.{ifIndex}|integer|{ifIndex}")  # ifIndex
        output.append(f"1.3.6.1.2.1.2.2.1.2.{ifIndex}|octetstring|GigabitEthernet1/0/{ifIndex}")  # ifDescr
        output.append(f"1.3.6.1.2.1.2.2.1.3.{ifIndex}|integer|6")  # ifType (ethernetCsmacd)
        output.append(f"1.3.6.1.2.1.2.2.1.4.{ifIndex}|integer|1500")  # ifMtu
        output.append(f"1.3.6.1.2.1.2.2.1.5.{ifIndex}|gauge|1000000000")  # ifSpeed (1 Gbps)
        
        # MAC address
        mac = f"00:1e:bd:c2:{ifIndex:02x}:{random.randint(0, 255):02x}"
        output.append(f"1.3.6.1.2.1.2.2.1.6.{ifIndex}|octetstring|{mac}")  # ifPhysAddress
        
        # Admin and Operational status (randomly up/down for realism)
        admin_status = 1  # up
        oper_status = 1 if random.random() > 0.1 else 2  # 90% up
        output.append(f"1.3.6.1.2.1.2.2.1.7.{ifIndex}|integer|{admin_status}")  # ifAdminStatus
        output.append(f"1.3.6.1.2.1.2.2.1.8.{ifIndex}|integer|{oper_status}")  # ifOperStatus
        
        output.append(f"1.3.6.1.2.1.2.2.1.9.{ifIndex}|timeticks|{random.randint(1000000, 9999999)}")  # ifLastChange
        
        # Traffic counters (with realistic random values)
        in_octets = random.randint(1000000000, 9999999999)
        out_octets = random.randint(1000000000, 9999999999)
        in_pkts = random.randint(10000000, 99999999)
        out_pkts = random.randint(10000000, 99999999)
        
        output.append(f"1.3.6.1.2.1.2.2.1.10.{ifIndex}|counter32|{in_octets}")  # ifInOctets
        output.append(f"1.3.6.1.2.1.2.2.1.11.{ifIndex}|counter32|{in_pkts}")  # ifInUcastPkts
        output.append(f"1.3.6.1.2.1.2.2.1.12.{ifIndex}|counter32|{random.randint(0, 100000)}")  # ifInNUcastPkts
        output.append(f"1.3.6.1.2.1.2.2.1.13.{ifIndex}|counter32|{random.randint(0, 1000)}")  # ifInDiscards
        output.append(f"1.3.6.1.2.1.2.2.1.14.{ifIndex}|counter32|{random.randint(0, 500)}")  # ifInErrors
        output.append(f"1.3.6.1.2.1.2.2.1.15.{ifIndex}|counter32|{random.randint(0, 100)}")  # ifInUnknownProtos
        
        output.append(f"1.3.6.1.2.1.2.2.1.16.{ifIndex}|counter32|{out_octets}")  # ifOutOctets
        output.append(f"1.3.6.1.2.1.2.2.1.17.{ifIndex}|counter32|{out_pkts}")  # ifOutUcastPkts
        output.append(f"1.3.6.1.2.1.2.2.1.18.{ifIndex}|counter32|{random.randint(0, 100000)}")  # ifOutNUcastPkts
        output.append(f"1.3.6.1.2.1.2.2.1.19.{ifIndex}|counter32|{random.randint(0, 1000)}")  # ifOutDiscards
        output.append(f"1.3.6.1.2.1.2.2.1.20.{ifIndex}|counter32|{random.randint(0, 500)}")  # ifOutErrors
        output.append(f"1.3.6.1.2.1.2.2.1.21.{ifIndex}|gauge|0")  # ifOutQLen
        
        # IF-MIB extensions (ifXTable - high capacity counters)
        output.append(f"1.3.6.1.2.1.31.1.1.1.1.{ifIndex}|octetstring|Gi1/0/{ifIndex}")  # ifName
        output.append(f"1.3.6.1.2.1.31.1.1.1.2.{ifIndex}|counter32|{random.randint(0, 10000)}")  # ifInMulticastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.3.{ifIndex}|counter32|{random.randint(0, 5000)}")  # ifInBroadcastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.4.{ifIndex}|counter32|{random.randint(0, 10000)}")  # ifOutMulticastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.5.{ifIndex}|counter32|{random.randint(0, 5000)}")  # ifOutBroadcastPkts
        
        # High capacity counters (64-bit)
        output.append(f"1.3.6.1.2.1.31.1.1.1.6.{ifIndex}|counter64|{in_octets * 100}")  # ifHCInOctets
        output.append(f"1.3.6.1.2.1.31.1.1.1.7.{ifIndex}|counter64|{in_pkts}")  # ifHCInUcastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.8.{ifIndex}|counter64|{random.randint(0, 10000)}")  # ifHCInMulticastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.9.{ifIndex}|counter64|{random.randint(0, 5000)}")  # ifHCInBroadcastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.10.{ifIndex}|counter64|{out_octets * 100}")  # ifHCOutOctets
        output.append(f"1.3.6.1.2.1.31.1.1.1.11.{ifIndex}|counter64|{out_pkts}")  # ifHCOutUcastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.12.{ifIndex}|counter64|{random.randint(0, 10000)}")  # ifHCOutMulticastPkts
        output.append(f"1.3.6.1.2.1.31.1.1.1.13.{ifIndex}|counter64|{random.randint(0, 5000)}")  # ifHCOutBroadcastPkts
        
        output.append(f"1.3.6.1.2.1.31.1.1.1.15.{ifIndex}|gauge|1000000000")  # ifHighSpeed (Mbps)
        output.append(f"1.3.6.1.2.1.31.1.1.1.18.{ifIndex}|octetstring|Port {ifIndex}")  # ifAlias
        
        # EtherLike-MIB (dot3)
        output.append(f"1.3.6.1.2.1.10.7.2.1.19.{ifIndex}|integer|3")  # dot3StatsDuplexStatus (fullDuplex)
        
        output.append("")
    
    # =======================================================
    # IP GROUP
    # =======================================================
    output.append("# ============================================================")
    output.append("# IP GROUP (1.3.6.1.2.1.4)")
    output.append("# ============================================================")
    output.append("")
    
    output.append("1.3.6.1.2.1.4.1.0|integer|1")  # ipForwarding
    output.append("1.3.6.1.2.1.4.2.0|integer|64")  # ipDefaultTTL
    output.append("1.3.6.1.2.1.4.3.0|counter32|" + str(random.randint(100000000, 999999999)))  # ipInReceives
    output.append("1.3.6.1.2.1.4.4.0|counter32|" + str(random.randint(0, 10000)))  # ipInHdrErrors
    output.append("1.3.6.1.2.1.4.5.0|counter32|" + str(random.randint(0, 1000)))  # ipInAddrErrors
    output.append("1.3.6.1.2.1.4.6.0|counter32|" + str(random.randint(0, 500)))  # ipForwDatagrams
    output.append("1.3.6.1.2.1.4.7.0|counter32|0")  # ipInUnknownProtos
    output.append("1.3.6.1.2.1.4.8.0|counter32|" + str(random.randint(0, 100)))  # ipInDiscards
    output.append("1.3.6.1.2.1.4.9.0|counter32|" + str(random.randint(100000000, 999999999)))  # ipInDelivers
    output.append("1.3.6.1.2.1.4.10.0|counter32|" + str(random.randint(100000000, 999999999)))  # ipOutRequests
    output.append("")
    
    # =======================================================
    # TCP/UDP GROUPS
    # =======================================================
    output.append("# ============================================================")
    output.append("# TCP GROUP (1.3.6.1.2.1.6)")
    output.append("# ============================================================")
    output.append("")
    
    output.append("1.3.6.1.2.1.6.1.0|integer|4")  # tcpRtoAlgorithm
    output.append("1.3.6.1.2.1.6.2.0|integer|200")  # tcpRtoMin
    output.append("1.3.6.1.2.1.6.3.0|integer|120000")  # tcpRtoMax
    output.append("1.3.6.1.2.1.6.4.0|integer|-1")  # tcpMaxConn
    output.append("1.3.6.1.2.1.6.5.0|counter32|" + str(random.randint(1000000, 9999999)))  # tcpActiveOpens
    output.append("1.3.6.1.2.1.6.6.0|counter32|" + str(random.randint(100000, 999999)))  # tcpPassiveOpens
    output.append("1.3.6.1.2.1.6.7.0|counter32|" + str(random.randint(1000, 9999)))  # tcpAttemptFails
    output.append("1.3.6.1.2.1.6.8.0|counter32|" + str(random.randint(100, 999)))  # tcpEstabResets
    output.append("1.3.6.1.2.1.6.9.0|gauge|" + str(random.randint(50, 500)))  # tcpCurrEstab
    output.append("1.3.6.1.2.1.6.10.0|counter32|" + str(random.randint(100000000, 999999999)))  # tcpInSegs
    output.append("1.3.6.1.2.1.6.11.0|counter32|" + str(random.randint(100000000, 999999999)))  # tcpOutSegs
    output.append("1.3.6.1.2.1.6.12.0|counter32|" + str(random.randint(1000, 9999)))  # tcpRetransSegs
    output.append("")
    
    output.append("# ============================================================")
    output.append("# UDP GROUP (1.3.6.1.2.1.7)")
    output.append("# ============================================================")
    output.append("")
    
    output.append("1.3.6.1.2.1.7.1.0|counter32|" + str(random.randint(10000000, 99999999)))  # udpInDatagrams
    output.append("1.3.6.1.2.1.7.2.0|counter32|" + str(random.randint(0, 1000)))  # udpNoPorts
    output.append("1.3.6.1.2.1.7.3.0|counter32|" + str(random.randint(0, 500)))  # udpInErrors
    output.append("1.3.6.1.2.1.7.4.0|counter32|" + str(random.randint(10000000, 99999999)))  # udpOutDatagrams
    output.append("")
    
    # =======================================================
    # CISCO-SPECIFIC MIBs
    # =======================================================
    output.append("# ============================================================")
    output.append("# CISCO SPECIFIC (1.3.6.1.4.1.9)")
    output.append("# ============================================================")
    output.append("")
    
    # CPU metrics (multiple CPUs)
    for cpu_idx in range(1, 5):
        cpu_usage = random.randint(10, 80)
        output.append(f"1.3.6.1.4.1.9.9.109.1.1.1.1.3.{cpu_idx}|gauge|{cpu_usage}")  # cpmCPUTotal1min
        output.append(f"1.3.6.1.4.1.9.9.109.1.1.1.1.4.{cpu_idx}|gauge|{cpu_usage + random.randint(-5, 5)}")  # cpmCPUTotal5min
        output.append(f"1.3.6.1.4.1.9.9.109.1.1.1.1.5.{cpu_idx}|gauge|{cpu_usage + random.randint(-10, 10)}")  # cpmCPUTotal15min
    
    output.append("")
    
    # Memory pools
    for mem_idx in range(1, 3):
        total_mem = random.randint(2000000000, 4000000000)
        used_mem = int(total_mem * random.uniform(0.4, 0.8))
        output.append(f"1.3.6.1.4.1.9.9.48.1.1.1.5.{mem_idx}|gauge|{used_mem}")  # ciscoMemoryPoolUsed
        output.append(f"1.3.6.1.4.1.9.9.48.1.1.1.6.{mem_idx}|gauge|{total_mem - used_mem}")  # ciscoMemoryPoolFree
        output.append(f"1.3.6.1.4.1.9.9.48.1.1.1.7.{mem_idx}|gauge|{total_mem}")  # ciscoMemoryPoolLargestFree
    
    output.append("")
    
    # Temperature sensors (8 sensors)
    for sensor_idx in range(1, 9):
        temp = random.randint(25, 65)
        output.append(f"1.3.6.1.4.1.9.9.13.1.3.1.3.{sensor_idx}|gauge|{temp}")  # ciscoEnvMonTemperatureValue
        output.append(f"1.3.6.1.4.1.9.9.13.1.3.1.4.{sensor_idx}|integer|1")  # ciscoEnvMonTemperatureState (normal)
        output.append(f"1.3.6.1.4.1.9.9.13.1.3.1.2.{sensor_idx}|octetstring|TempSensor{sensor_idx}")  # description
    
    output.append("")
    
    # Fan sensors (6 fans)
    for fan_idx in range(1, 7):
        output.append(f"1.3.6.1.4.1.9.9.13.1.4.1.3.{fan_idx}|integer|1")  # ciscoEnvMonFanState (normal)
        output.append(f"1.3.6.1.4.1.9.9.13.1.4.1.2.{fan_idx}|octetstring|Fan{fan_idx}")  # description
    
    output.append("")
    
    # Power supplies (4 PSUs)
    for psu_idx in range(1, 5):
        output.append(f"1.3.6.1.4.1.9.9.13.1.5.1.3.{psu_idx}|integer|1")  # ciscoEnvMonSupplyState (normal)
        output.append(f"1.3.6.1.4.1.9.9.13.1.5.1.2.{psu_idx}|octetstring|PowerSupply{psu_idx}")  # description
    
    output.append("")
    
    # Entity MIB - Hardware components
    output.append("# ============================================================")
    output.append("# ENTITY MIB (1.3.6.1.2.1.47)")
    output.append("# ============================================================")
    output.append("")
    
    output.append("1.3.6.1.2.1.47.1.1.1.1.2.1|octetstring|Cisco 7200 Series Router")  # entPhysicalDescr (chassis)
    output.append("1.3.6.1.2.1.47.1.1.1.1.7.1|octetstring|C7200-ADVIPSERVICESK9")  # entPhysicalName
    output.append("1.3.6.1.2.1.47.1.1.1.1.11.1|octetstring|CAT1234ABCD")  # entPhysicalSerialNum
    output.append("1.3.6.1.2.1.47.1.1.1.1.13.1|octetstring|Cisco Systems")  # entPhysicalModelName
    
    output.append("")
    output.append(f"# Total OIDs: ~{48 * 35 + 100} (estimated)")
    
    return "\n".join(output)

if __name__ == "__main__":
    print("Generating rich SNMPREC file...")
    content = generate_snmprec()
    
    with open("sample-rich.snmprec", "w") as f:
        f.write(content)
    
    # Count lines
    lines = content.split("\n")
    oid_lines = [l for l in lines if l and not l.startswith("#") and "|" in l]
    
    print(f"✅ Generated sample-rich.snmprec")
    print(f"   Total lines: {len(lines)}")
    print(f"   OID entries: {len(oid_lines)}")
    print(f"   Interfaces: 48")
    print(f"   Estimated metrics per interface: ~35")
    print(f"   Total interface metrics: ~{48 * 35}")
    print(f"   System metrics: ~{len(oid_lines) - (48 * 35)}")
