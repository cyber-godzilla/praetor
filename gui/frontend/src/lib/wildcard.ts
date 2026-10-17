export type MatchScope = "substring" | "whole";

export interface MatchSpan {
  start: number;
  end: number;
}

export interface WildcardPattern {
  matches(text: string): boolean;
  findAll(text: string): MatchSpan[];
}

function quoteRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function wildcardSource(pattern: string): string {
  const runes = Array.from(pattern);
  let source = "";
  for (let i = 0; i < runes.length; i++) {
    const rune = runes[i];
    if (rune === "\\" && i + 1 < runes.length && ["*", "?", "\\"].includes(runes[i + 1])) {
      source += quoteRegex(runes[++i]);
      continue;
    }
    if (rune === "*") source += ".*?";
    else if (rune === "?") source += ".";
    else source += quoteRegex(rune);
  }
  return source;
}

export function compileWildcard(pattern: string, scope: MatchScope): WildcardPattern {
  const empty = pattern === "";
  const onlyStar = pattern === "*";
  const source = scope === "whole" ? `^(?:${wildcardSource(pattern)})$` : wildcardSource(pattern);

  return {
    matches(text: string): boolean {
      return !empty && new RegExp(source, "iu").test(text);
    },
    findAll(text: string): MatchSpan[] {
      if (empty) return [];
      if (onlyStar) return text === "" ? [] : [{ start: 0, end: text.length }];
      const spans: MatchSpan[] = [];
      for (const match of text.matchAll(new RegExp(source, "giu"))) {
        const start = match.index;
        const end = start + match[0].length;
        if (start !== end) spans.push({ start, end });
      }
      return spans;
    },
  };
}
