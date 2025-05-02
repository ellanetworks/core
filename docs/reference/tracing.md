---
description: Reference for tracing in Ella Core.
---

# Tracing

Ella Core supports tracing using [OpenTelemetry](https://opentelemetry.io/). This allows users to collect and export traces from Ella Core to a tracing backend for analysis and visualization.

## API Interface

Ella Core captures traces for each API call, allowing users to monitor and analyze the performance of API requests. This includes information about the request time, as well as any errors that may occur. Each API trace that incurs database access includes spans for the database operations.

<figure markdown="span">
  ![Tracing](../images/tracing_api.png){ width="800" }
  <figcaption>Tracing Ella Core's API interfaces.</figcaption>
</figure>

## NGAP Interface (N2)

Ella Core captures traces from each NGAP communication with the RAN, allowing users to monitor and analyze the performance of NGAP messages. This includes information about the message type. Each NGAP trace that includes NAS message handling includes spans for the NAS operations.

<figure markdown="span">
  ![Tracing](../images/tracing_ngap.png){ width="800" }
  <figcaption>Tracing Ella Core's NGAP interfaces.</figcaption>
</figure>

## Configuration

For more information on configuring tracing in Ella Core, refer to the [Configuration File](config_file.md) documentation.
