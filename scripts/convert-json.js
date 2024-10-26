import fs from "fs";
import { promptFormats } from "./prompt-formats.js";

// Turn mustache templates to Go templates
function convertTemplateVars(template) {
  return template.replace(/{{(\w+)}}/g, (_, varName) => {
    const capitalizedVar = varName.charAt(0).toUpperCase() + varName.slice(1);
    return `{{.${capitalizedVar}}}`;
  });
}

try {
  const jsonStr = convertTemplateVars(JSON.stringify(promptFormats, null, 2));

  fs.writeFile("prompt-formats.json", jsonStr, (err) => {
    if (err) throw err;
    console.log("JSON data has been written to prompt-formats.json");
  });
} catch (err) {
  console.error("Error parsing JSON data:", err);
}
