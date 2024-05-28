module.exports = {
  async redirects() {
    return [
      {
        source: '/',
        destination: '/network-configuration',
        permanent: true,
      },
    ];
  },
  env: {
    UPF_CONFIG_PATH: process.env.UPF_CONFIG_PATH,
    GNB_CONFIG_PATH: process.env.GNB_CONFIG_PATH,
    WEBUI_ENDPOINT: process.env.WEBUI_ENDPOINT,
  },
};
