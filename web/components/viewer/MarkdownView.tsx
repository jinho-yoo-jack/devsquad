"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

/** Plan·Deliverable 원문 렌더. 코드 하이라이트는 Phase 2 (rehype-highlight). */
export function MarkdownView({ content, className }: { content: string; className?: string }) {
  return (
    <div className={`markdown text-sm leading-6 ${className ?? ""}`}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
}
