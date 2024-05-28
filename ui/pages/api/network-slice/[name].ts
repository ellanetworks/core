import { NextApiRequest, NextApiResponse } from "next";


const WEBUI_ENDPOINT = process.env.WEBUI_ENDPOINT;


export default async function handleNetworkSlice(req: NextApiRequest, res: NextApiResponse) {
  switch (req.method) {
    case "GET":
      return handleGET(req, res);
    case "POST":
      return handlePOST(req, res);
    case "DELETE":
      return handleDELETE(req, res);
    default:
      res.setHeader("Allow", ["GET", "POST","PUT", "DELETE"]);
      res.status(405).end(`Method ${req.method} Not Allowed`);
  }
}

function isValidName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

/**
 * @openapi
 * /api/network-slice/{sliceName}:
 *   get:
 *     tags:
 *       - Network Slices
 *     description: Returns the network slice
 *     parameters:
 *       - name: sliceName
 *         in: path
 *         required: true
 *         type: string
 *     responses:
 *       200:
 *         description: Network slice
 */
async function handleGET(req: NextApiRequest, res: NextApiResponse) {
  const { name } = req.query

  if (typeof name !== 'string') {
    res.status(400).json({ error: "Invalid name provided." });
    return;
  }

  if (!isValidName(name)) {
    res.status(400).json({ error: "Invalid name provided." });
    return;
  }

  const url = `${WEBUI_ENDPOINT}/config/v1/network-slice/${name}`;

  try {
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });


    if (!response.ok) {
      res.status(response.status).json({ error: "Error retrieving network slice." });
      return;
    }

    const data = await response.json();
    res.status(200).json(data);

  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while getting the network slice",
    });
  }
}



/**
 * @openapi
 * /api/network-slice/{sliceName}:
 *   post:
 *     tags:
 *       - Network Slices
 *     description: Create a new network slice
 *     parameters:
 *       - name: sliceName
 *         in: path
 *         required: true
 *         schema:
 *           type: string
 *     requestBody:
 *      description: Network slice object
 *      required: true
 *      content:
 *        application/json:
 *          schema:
 *            type: object
 *            properties:
 *              slice-id:
 *                type: object
 *                properties:
 *                  sst:
 *                    type: string
 *                  sd:
 *                    type: string
 *              site-device-group:
 *                type: array
 *                items:
 *                  type: string
 *              site-info:
 *                type: object
 *                properties:
 *                  site-name:
 *                    type: string
 *                  plmn:
 *                    type: object
 *                    properties:
 *                      mcc:
 *                        type: string
 *                      mnc:
 *                        type: string
 *                  gNodeBs:
 *                    type: array
 *                    items:
 *                      type: string
 *                  upf:
 *                    type: object
 *                    properties:
 *                      upf-name:
 *                        type: string
 *                      upf-port:
 *                        type: string
 *     responses:
 *       200:
 *         description: Network slice created
 *       400:
 *         description: Invalid network slice content
 *       500:
 *         description: Error creating network slice
 */
async function handlePOST(req: NextApiRequest, res: NextApiResponse) {
  const { name } = req.query

  if (typeof name !== 'string') {
    res.status(400).json({ error: "Invalid name provided." });
    return;
  }

  if (!isValidName(name)) {
    res.status(400).json({ error: "Invalid name provided." });
    return;
  }
  
  const url = `${WEBUI_ENDPOINT}/config/v1/network-slice/${name}`;

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(req.body),
    });

    if (!response.ok) {
      throw new Error(`Error creating network slice. Error code: ${response.status}`);
    }

    res.status(200).json({ message: "Network Slice created successfully" });
  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while creating the network slice",
    });
  }
}

/**
 * @openapi
 * /api/network-slice/{sliceName}:
 *   delete:
 *     tags:
 *       - Network Slices
 *     description: Delete an existing network slice
 *     parameters:
 *       - name: sliceName
 *         in: path
 *         required: true
 *         schema:
 *           type: string
 *     responses:
 *       202:
 *         description: Network slice deleted successfully
 *       400:
 *         description: Invalid network slice name provided
 *       500:
 *         description: Error deleting network slice
 */
async function handleDELETE(req: NextApiRequest, res: NextApiResponse) {
    const { name } = req.query

    if (typeof name !== 'string') {
      res.status(400).json({ error: "Invalid name provided." });
      return;
    }
  
    if (!isValidName(name)) {
      res.status(400).json({ error: "Invalid name provided." });
      return;
    }
  
    const url = `${WEBUI_ENDPOINT}/config/v1/network-slice/${name}`;
  
    try {
      const response = await fetch(url, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
        },
      });
  
      if (!response.ok) {
        throw new Error(
          `Error deleting network slice. Error code: ${response.status}`
        );
      }
  
      res.status(202).send('Deleted successfully');
    } catch (error) {
      console.error("Error details:", error);
      res.status(500).json({
        error: "An error occurred while deleting the network slice",
      });
    }
  }
  