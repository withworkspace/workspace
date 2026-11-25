// The keys of Documents and of a space's page, as the help says them.

export const documentsKeys: [string, string][] = [
  ["n", "Start a page"],
  ["N", "Start a page as one of the workspace's kinds of document"],
  ["/", "Search the documents"],
  ["j / k", "Move between spaces and pages"],
  ["⏎ or o", "Open it"],
];

export const spaceKeys: [string, string][] = [
  ["j / k", "Move between documents: the one picked is shown beside the list"],
  ["⏎ or o", "Read it"],
  ["/", "Search the space"],
  ["n", "Start a page here"],
  ["esc", "Clear the search, or back to Documents"],
];
