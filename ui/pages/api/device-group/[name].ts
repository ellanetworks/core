import { NextApiRequest, NextApiResponse } from "next";


const WEBUI_ENDPOINT = process.env.WEBUI_ENDPOINT;


export default async function handleDeviceGroup(req: NextApiRequest, res: NextApiResponse) {
  
  switch (req.method) {
    case "GET":
      return handleGET(req, res);
    case "POST":
      return handlePOST(req, res);
    case "DELETE":
      return handleDELETE(req, res);
    default:
      res.setHeader("Allow", ["GET", "POST", "DELETE"]);
      res.status(405).end(`Method ${req.method} Not Allowed`);
  }
}

function isValidName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}


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
  
  const url = `${WEBUI_ENDPOINT}/config/v1/device-group/${name}`;

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(req.body),
    });

    if (!response.ok) {
      throw new Error(`Error creating device group. Error code: ${response.status}`);
    }

    res.status(200).json({ message: "Device Group created successfully" });
  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while creating the device group",
    });
  }
}

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
  
  const url = `${WEBUI_ENDPOINT}/config/v1/device-group/${name}`;

  try {
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      res.status(response.status).json({ error: "Error retrieving device group." });
      return;
    }

    const data = await response.json();
    res.status(200).json(data);
  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while retrieving the device group",
    });
  }
}

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
  
  const url = `${WEBUI_ENDPOINT}/config/v1/device-group/${name}`;

  try {
    const response = await fetch(url, {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!response.ok) {
      throw new Error(`Error deleting device group. Error code: ${response.status}`);
    }

    res.status(200).json({ message: "Device Group deleted successfully" });
  } catch (error) {
    console.error("Error details:", error);
    res.status(500).json({
      error: "An error occurred while deleting the device group",
    });
  }
}
