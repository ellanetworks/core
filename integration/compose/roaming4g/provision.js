const k = "465B5CE8B199B49FAA5F0A2EE238A6BC";
const opc = "E8ED289DEBA952E4283B54E88E6183CA";
const gbps = { value: NumberInt(1), unit: NumberInt(3) };

function apn(name, lbo, type) {
  return {
    name: name,
    type: NumberInt(type),
    lbo_roaming_allowed: lbo,
    qos: {
      index: NumberInt(9),
      arp: {
        priority_level: NumberInt(8),
        pre_emption_capability: NumberInt(1),
        pre_emption_vulnerability: NumberInt(1),
      },
    },
    ambr: { downlink: gbps, uplink: gbps },
    pcc_rule: [],
    _id: new ObjectId(),
  };
}

function subscriber(imsi, msisdn, internetType) {
  return {
    schema_version: NumberInt(1),
    imsi: imsi,
    msisdn: [msisdn],
    imeisv: [],
    mme_host: [],
    mm_realm: [],
    purge_flag: [],
    slice: [
      {
        sst: NumberInt(1),
        default_indicator: true,
        session: [apn("internet", true, internetType), apn("corporate", true, 3)],
        _id: new ObjectId(),
      },
    ],
    security: { k: k, op: null, opc: opc, amf: "8000", sqn: NumberLong(64) },
    ambr: { downlink: gbps, uplink: gbps },
    access_restriction_data: NumberInt(32),
    network_access_mode: NumberInt(0),
    subscriber_status: NumberInt(0),
    operator_determined_barring: NumberInt(0),
    subscribed_rau_tau_timer: NumberInt(12),
    __v: 0,
  };
}

db = db.getSiblingDB("open5gs");
const ipv4OrIPv6 = { "001010000000112": 4 };
for (let i = 1; i <= 19; i++) {
  const n = String(i).padStart(2, "0");
  const imsi = "0010100000001" + n;
  db.subscribers.replaceOne(
    { imsi: imsi },
    subscriber(imsi, "155501000" + n, ipv4OrIPv6[imsi] || 3),
    { upsert: true },
  );
}
print("provisioned " + db.subscribers.countDocuments({}) + " subscribers");
