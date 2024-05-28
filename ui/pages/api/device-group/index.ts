import { NextApiRequest, NextApiResponse } from "next";


const WEBUI_ENDPOINT = process.env.WEBUI_ENDPOINT;

/**
 * @openapi
 * /api/device-group/:
 *   get:
 *     tags:
 *       - Device Groups
 *     description: Returns the list of device groups
 *     responses:
 *       200:
 *         description: List of device groups
 *       500:
 *        description: Error retrieving device groups
 */
export default async function handleDeviceGroup(req: NextApiRequest, res: NextApiResponse) {
  switch (req.method) {
    case "GET":
      return handleGET(req, res);
    default:
      res.setHeader("Allow", ["GET"]);
      res.status(405).end(`Method ${req.method} Not Allowed.`);
  }
}


async function handleGET(req: NextApiRequest, res: NextApiResponse) {
  const url = `${WEBUI_ENDPOINT}/config/v1/device-group/`;
  try {
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(
        `Error retrieving device group. Error code: ${response.status}`
      );
    }
    const data = await response.json();
    res.status(200).json(data);
  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while retrieving the device groups.",
    });
  }
}
