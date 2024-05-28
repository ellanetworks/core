import { NextApiRequest, NextApiResponse } from "next";
import fs from 'fs';
import path from 'path';

export default async function gnb(req: NextApiRequest, res: NextApiResponse) {
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
        const configPath = path.resolve(process.env.GNB_CONFIG_PATH || '');

        if (fs.existsSync(configPath) && fs.statSync(configPath).isFile()) {
            const gnbList = JSON.parse(fs.readFileSync(configPath, 'utf-8'));
            res.status(200).json(gnbList);
        } else {
            console.error('GNB config file does not exist or is not a file');
            res.status(200).json([]);
        }
    } catch (error) {
        console.error('Failed to read GNB config file:', error);
        res.status(500).json({ message: "Error reading GNB config file" });
    }
}