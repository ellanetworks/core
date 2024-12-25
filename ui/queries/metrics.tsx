import { HTTPStatus } from "@/queries/utils";

export const getMetrics = async () => {
  const response = await fetch(`/api/v1/metrics`, {
    method: "GET",
  });
  let respData;
  // Metrics are in Prometheus Format Ex.
  //   # HELP go_gc_duration_seconds A summary of the wall-time pause (stop-the-world) duration in garbage collection cycles.
  // # TYPE go_gc_duration_seconds summary
  // go_gc_duration_seconds{quantile="0"} 2.476e-05
  // go_gc_duration_seconds{quantile="0.25"} 4.6824e-05

  if (!response.ok) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`);
  }

  return response.text();
};
