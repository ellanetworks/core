const imsi = "001010000000001";
const sub = {
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
      default_indicator: true,
      session: [
        {
          name: "internet",
          type: NumberInt(3),
          lbo_roaming_allowed: true,
          qos: {
            index: NumberInt(9),
            arp: {
              priority_level: NumberInt(8),
              pre_emption_capability: NumberInt(1),
              pre_emption_vulnerability: NumberInt(2),
            },
          },
          ambr: {
            downlink: { value: NumberInt(1000000000), unit: NumberInt(0) },
            uplink: { value: NumberInt(1000000000), unit: NumberInt(0) },
          },
          pcc_rule: [],
          _id: new ObjectId(),
        },
      ],
      _id: new ObjectId(),
    },
  ],
  security: {
    k: "465B5CE8B199B49FAA5F0A2EE238A6BC",
    op: null,
    opc: "E8ED289DEBA952E4283B54E88E6183CA",
    amf: "8000",
    sqn: NumberLong(0),
  },
  ambr: {
    downlink: { value: NumberInt(1000000000), unit: NumberInt(0) },
    uplink: { value: NumberInt(1000000000), unit: NumberInt(0) },
  },
  access_restriction_data: 32,
  network_access_mode: 0,
  subscriber_status: 0,
  operator_determined_barring: 0,
  subscribed_rau_tau_timer: 12,
  __v: 0,
};
db = db.getSiblingDB("open5gs");
db.subscribers.replaceOne({ imsi: imsi }, sub, { upsert: true });
printjson(db.subscribers.findOne({ imsi: imsi }, { imsi: 1, security: 1, "slice.session.lbo_roaming_allowed": 1 }));
