import { NextApiRequest, NextApiResponse } from "next";
import fs from 'fs';
import path from 'path';

export default async function upf(req: NextApiRequest, res: NextApiResponse) {
    switch (req.method) {
        case 'GET':
            await handleGET(req, res);
            break;
        default:
            res.setHeader('Allow', 'GET');
            res.status(405).json({ message: 'Method Not Allowed. Only GET requests are supported.' });
            break;
    }
}

async function handleGET(req: NextApiRequest, res: NextApiResponse) {
    try {
        const configPath = path.resolve(process.env.UPF_CONFIG_PATH || '');

        if (fs.existsSync(configPath) && fs.statSync(configPath).isFile()) {
            const upfList = JSON.parse(fs.readFileSync(configPath, 'utf-8'));
            res.status(200).json(upfList);
        } else {
            console.error('UPF config file does not exist or is not a file');
            res.status(200).json([]);
        }
    } catch (error) {
        console.error('Failed to read UPF config file:', error);
        res.status(500).json({ message: "Error reading UPF config file" });
    }
}