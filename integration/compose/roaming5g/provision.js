const security = {
  k: "465B5CE8B199B49FAA5F0A2EE238A6BC",
  op: null,
  opc: "E8ED289DEBA952E4283B54E88E6183CA",
  amf: "8000",
  sqn: NumberLong(0),
};

function session(name, lbo, type) {
  return {
    name: name,
    type: NumberInt(type || 3),
    lbo_roaming_allowed: lbo,
    qos: {
      index: NumberInt(9),
      arp: {
        priority_level: NumberInt(8),
        pre_emption_capability: NumberInt(1),
        pre_emption_vulnerability: NumberInt(2),
      },
    },
    ambr: {
      downlink: { value: NumberInt(300), unit: NumberInt(2) },
      uplink: { value: NumberInt(150), unit: NumberInt(2) },
    },
    pcc_rule: [],
    _id: new ObjectId(),
  };
}

db = db.getSiblingDB("open5gs");

for (let i = 1; i <= 9; i++) {
  const imsi = "00101000000020" + i;
  db.subscribers.replaceOne(
    { imsi: imsi },
    {
      schema_version: NumberInt(1),
      imsi: imsi,
      msisdn: [],
      imeisv: [],
      mme_host: [],
      mm_realm: [],
      purge_flag: [],
      slice: [
        {
          sst: NumberInt(1),
          sd: "102030",
          default_indicator: true,
          session: [session("internet", true), session("corp", false)],
          _id: new ObjectId(),
        },
      ],
      security: security,
      ambr: {
        downlink: { value: NumberInt(400), unit: NumberInt(2) },
        uplink: { value: NumberInt(200), unit: NumberInt(2) },
      },
      access_restriction_data: 32,
      network_access_mode: 0,
      subscriber_status: 0,
      operator_determined_barring: 0,
      subscribed_rau_tau_timer: 12,
      __v: 0,
    },
    { upsert: true },
  );
}

for (let i = 1; i <= 4; i++) {
  const imsi = "00101000000021" + i;
  db.subscribers.replaceOne(
    { imsi: imsi },
    {
      schema_version: NumberInt(1),
      imsi: imsi,
      msisdn: ["1555020001" + i],
      imeisv: [],
      mme_host: [],
      mm_realm: [],
      purge_flag: [],
      slice: [
        {
          sst: NumberInt(1),
          default_indicator: true,
          session: [session("internet", true, 1)],
          _id: new ObjectId(),
        },
      ],
      security: security,
      ambr: {
        downlink: { value: NumberInt(400), unit: NumberInt(2) },
        uplink: { value: NumberInt(200), unit: NumberInt(2) },
      },
      access_restriction_data: 32,
      network_access_mode: 0,
      subscriber_status: 0,
      operator_determined_barring: 0,
      subscribed_rau_tau_timer: 12,
      __v: 0,
    },
    { upsert: true },
  );
}

print("provisioned " + db.subscribers.countDocuments({}) + " subscribers");
