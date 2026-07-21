export default {
  extends: ["@commitlint/config-conventional"],
  rules: {
    // Scopes match the top-level workspace areas, so a commit message
    // documents which part of the polyglot repo it touches.
    "scope-enum": [
      2,
      "always",
      ["foundation", "apps", "packages", "services", "sdk", "specs", "tools", "ci", "docs"],
    ],
  },
};
