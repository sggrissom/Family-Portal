import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import ts from "typescript";

const sourceRoot = path.resolve(process.argv[2] ?? "frontend");
const sourceExtensions = new Set([".ts", ".tsx"]);

function listSourceFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) return listSourceFiles(entryPath);
    return sourceExtensions.has(path.extname(entry.name)) ? [entryPath] : [];
  });
}

function countTopLevelRules(css) {
  let depth = 0;
  let rules = 0;
  let quote = null;
  let inComment = false;

  for (let index = 0; index < css.length; index++) {
    const character = css[index];
    const next = css[index + 1];

    if (inComment) {
      if (character === "*" && next === "/") {
        inComment = false;
        index++;
      }
      continue;
    }

    if (quote) {
      if (character === "\\") {
        index++;
      } else if (character === quote) {
        quote = null;
      }
      continue;
    }

    if (character === "/" && next === "*") {
      inComment = true;
      index++;
    } else if (character === '"' || character === "'") {
      quote = character;
    } else if (character === "{") {
      if (depth === 0) rules++;
      depth++;
    } else if (character === "}") {
      depth--;
      if (depth < 0) return { rules, balanced: false };
    }
  }

  return { rules, balanced: depth === 0 && !inComment && quote === null };
}

const failures = [];

for (const filePath of listSourceFiles(sourceRoot)) {
  const sourceText = fs.readFileSync(filePath, "utf8");
  const sourceFile = ts.createSourceFile(
    filePath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    filePath.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS
  );

  function visit(node) {
    if (
      ts.isCallExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === "block"
    ) {
      const argument = node.arguments[0];
      const position = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
      const location = `${path.relative(process.cwd(), filePath)}:${position.line + 1}:${position.character + 1}`;

      if (!argument || !ts.isNoSubstitutionTemplateLiteral(argument)) {
        failures.push(`${location} block(...) must contain one static CSS template literal.`);
      } else {
        const result = countTopLevelRules(argument.text);
        if (!result.balanced) {
          failures.push(
            `${location} block(...) contains unbalanced CSS braces, quotes, or comments.`
          );
        } else if (result.rules !== 1) {
          failures.push(
            `${location} block(...) must contain exactly one top-level CSS rule; found ${result.rules}. ` +
              "Split each rule into its own block(...). A single @media rule may contain multiple nested rules."
          );
        }
      }
    }

    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
}

if (failures.length > 0) {
  console.error("\nCSS block validation failed:\n");
  for (const failure of failures) console.error(`  ERROR ${failure}`);
  console.error(
    `\nFound ${failures.length} invalid CSS block${failures.length === 1 ? "" : "s"}.\n`
  );
  process.exit(1);
}

console.log("CSS block validation passed.");
