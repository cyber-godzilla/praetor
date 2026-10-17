export type VariableErrorCode = "unresolved" | "invalid_name" | "unterminated";

export class VariableExpansionError extends Error {
  constructor(
    public readonly code: VariableErrorCode,
    public readonly variable: string,
    public readonly offset: number,
  ) {
    const message = code === "unresolved"
      ? `unknown variable ${JSON.stringify(variable)}`
      : code === "invalid_name"
        ? `invalid variable reference ${JSON.stringify(variable)}`
        : `unterminated variable reference at character ${offset + 1}`;
    super(message);
    this.name = "VariableExpansionError";
  }
}

export function validVariableName(name: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(name);
}

// Expand once. Inserted values and fallbacks remain data and are never scanned
// for nested references or executable input syntax.
export function expandVariables(input: string, values: Record<string, string> | null | undefined): string {
  let output = "";
  for (let i = 0; i < input.length; i++) {
    if (input[i] === "\\" && input[i + 1] === "$" && (input[i + 2] === "{" || input[i + 2] === "(")) {
      output += `$${input[i + 2]}`;
      i += 2;
      continue;
    }
    if (input[i] !== "$" || input[i + 1] !== "{") {
      output += input[i];
      continue;
    }
    const end = input.indexOf("}", i + 2);
    if (end < 0) throw new VariableExpansionError("unterminated", "", i);
    const reference = input.slice(i + 2, end);
    const separator = reference.indexOf(":");
    const name = separator < 0 ? reference : reference.slice(0, separator);
    const hasFallback = separator >= 0;
    const fallback = hasFallback ? reference.slice(separator + 1) : "";
    if (!validVariableName(name)) throw new VariableExpansionError("invalid_name", name, i);
    const present = Object.prototype.hasOwnProperty.call(values ?? {}, name);
    let value = present ? (values ?? {})[name] : undefined;
    if ((!present || value === "") && hasFallback) value = fallback;
    else if (!present) throw new VariableExpansionError("unresolved", name, i);
    output += value ?? "";
    i = end;
  }
  return output;
}
