export default {
  branches: ["main"],
  plugins: [
    "@semantic-release/commit-analyzer",
    "@semantic-release/release-notes-generator",
    ["@semantic-release/exec", { publishCmd: "sh ./scripts/release.sh ${nextRelease.gitTag}" }],
  ],
};
