# Contributing


## Development

To make contributions to this project, make sure you have [`Nodejs 18`](https://nodejs.org/) installed.

1. Clone the repository:

   ```shell
   git clone git@github.com:canonical/sdcore-nms.git
   ```

2. Navigate to the project directory:

   ```shell
   cd sdcore-nms
   ```

3. Install the dependencies:

   ```shell
   npm install
   ```

4. Run the development server:

   ```bash
   npm run dev
   ```

Open [http://localhost:3000](http://localhost:3000) with your browser to view the changes.

## Testing

This project uses [ESLint](https://eslint.org/) and [Prettier](https://prettier.io/) for linting and code formatting.

💡 We recommend using the [Prettier extension for VSCode](https://marketplace.visualstudio.com/items?itemName=esbenp.prettier-vscode) for easy on-save code formatting.

To run lint check:

```shell
npm run lint
```

## Build

To build the project:

```shell
npm run build
```

## Container image

Pack the rock

```bash
sudo snap install rockcraft --edge --classic
rockcraft pack -v
```

Move the rock to Docker's registry

```bash
sudo rockcraft.skopeo --insecure-policy copy oci-archive:sdcore-nms_0.2.0_amd64.rock docker-daemon:sdcore-nms:0.2.0
```

Run the NMS

```bash
docker run -p 3000:3000 sdcore-nms:0.2.0
```

You will have the NMS available in `http://localhost:3000`.
