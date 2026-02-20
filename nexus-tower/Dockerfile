FROM node:20-alpine

WORKDIR /workspace
COPY nexus-tower/package.json /workspace/package.json
RUN npm install

COPY nexus-tower /workspace

EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "5173"]
