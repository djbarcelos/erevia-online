import { defineRailway, github, project, service } from "railway/iac";
// This repository manages only its own resources in the environment. Other
// repositories export their own partial name.
// See https://docs.railway.com/infrastructure-as-code#multi-repo-projects
export const partial = "erevia";
export default defineRailway(() => {
  const erevia = service("erevia", {
    // dockerfilePath from CaC: "Dockerfile"
    // builder from CaC: "DOCKERFILE"
    // checkSuites: só publica quando o CI do GitHub (.github/workflows/ci.yml) passa.
    source: github("djbarcelos/erevia-online", { branch: "main", checkSuites: true }),
    healthcheck: "/healthz",
    healthcheckTimeout: 120,
    replicas: { "us-east4-eqdc4a": 1 },
  });

  return project("Erevia", {
    variables: { managed: false },
    resources: [erevia],
  });
});
