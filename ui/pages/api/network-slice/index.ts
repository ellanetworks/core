import { NextApiRequest, NextApiResponse } from "next";


const WEBUI_ENDPOINT = process.env.WEBUI_ENDPOINT;

/**
 * @openapi
 * /api/network-slice/:
 *   get:
 *     tags:
 *       - Network Slices
 *     description: Returns the list of network slices
 *     responses:
 *       200:
 *         description: List of network slices
 *       500:
 *        description: Error retrieving network slices
 */
export default async function handleNetworkSlice(req: NextApiRequest, res: NextApiResponse) {
  switch (req.method) {
    case "GET":
      return handleGET(req, res);
    default:
      res.setHeader("Allow", ["GET"]);
      res.status(405).end(`Method ${req.method} Not Allowed`);
  }
}


async function handleGET(req: NextApiRequest, res: NextApiResponse) {
  const url = `${WEBUI_ENDPOINT}/config/v1/network-slice/`;

  try {
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(
        `Error retrieving network slice. Error code: ${response.status}`
      );
    }

    const data = await response.json();

    res.status(200).json(data);
  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while retrieving the network slice",
    });
  }
}
