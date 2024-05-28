import { createSwaggerSpec } from 'next-swagger-doc';

export const getApiDocs = async () => {
  const spec = createSwaggerSpec({
    apiFolder: 'pages/api',
    definition: {
      openapi: '3.0.0',
      info: {
        title: '5G NMS API Documentation',
        version: '1.0',
      },
      components: {},
      security: [],
    },
  });
  return spec;
};
