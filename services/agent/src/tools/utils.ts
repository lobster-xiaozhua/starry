import { tool } from "@langchain/core/tools";
import { z } from "zod";

// 安全计算器：自实现词法+调度场+逆波兰求值，禁用 eval，
// 支持 + - * / % ^ 与括号、一元负号、小数，以及 sqrt/abs/min/max/round/pow/ln。
export const calculatorTool = tool(
  async ({ expression }) => {
    try {
      const value = evalExpr(expression);
      if (!Number.isFinite(value)) return "计算结果无效（溢出或非数值）。";
      return `= ${round(value)}`;
    } catch (e: any) {
      return `计算错误：${e?.message || e}。仅支持 + - * / % ^ 与括号，以及函数 sqrt/abs/min/max/round/pow/ln。`;
    }
  },
  {
    name: "calculator",
    description: "安全计算数学表达式。当用户需要精确计算时使用。",
    schema: z.object({
      expression: z.string().describe("数学表达式，例如 (1+2)*3^2 或 sqrt(16)+ln(1)"),
    }),
  },
);

export const currentTimeTool = tool(
  async () => {
    const now = new Date();
    return [
      `ISO: ${now.toISOString()}`,
      `本地: ${now.toString()}`,
      `UTC偏移(分钟): ${now.getTimezoneOffset()}`,
      `Unix: ${Math.floor(now.getTime() / 1000)}`,
    ].join("\n");
  },
  {
    name: "current_time",
    description: "返回当前日期与时间（ISO/本地/Unix），用于涉及「今天/现在」的问题。",
    schema: z.object({}),
  },
);

// ===== 简易安全表达式求值 =====

type Tok = { t: "num" | "op" | "fn" | "lp" | "rp" | "comma" | "fopen"; v: string };

const FUNCS: Record<string, (a: number[]) => number> = {
  sqrt: (a) => Math.sqrt(a[0]),
  abs: (a) => Math.abs(a[0]),
  ln: (a) => Math.log(a[0]),
  log: (a) => Math.log10(a[0]),
  round: (a) => Math.round(a[0]),
  min: (a) => Math.min(...a),
  max: (a) => Math.max(...a),
  pow: (a) => Math.pow(a[0], a[1]),
};

function tokenize(s: string): Tok[] {
  const toks: Tok[] = [];
  let i = 0;
  while (i < s.length) {
    const c = s[i];
    if (c === " " || c === "\t" || c === "\n") {
      i++;
      continue;
    }
    if (/[0-9.]/.test(c)) {
      let num = "";
      while (i < s.length && /[0-9.eE+\-]/.test(s[i])) {
        // 允许科学计数法 e±，但避免把普通减号吃掉
        if ((s[i] === "+" || s[i] === "-") && !/[eE]/.test(s[i - 1] ?? "")) break;
        num += s[i];
        i++;
      }
      if (!/^[0-9.]+([eE][+-]?[0-9]+)?$/.test(num)) throw new Error("数字格式错误");
      toks.push({ t: "num", v: num });
      continue;
    }
    if (/[a-zA-Z_]/.test(c)) {
      let name = "";
      while (i < s.length && /[a-zA-Z0-9_]/.test(s[i])) {
        name += s[i];
        i++;
      }
      toks.push({ t: "fn", v: name });
      continue;
    }
    if (c === "+" || c === "-" || c === "*" || c === "/" || c === "%" || c === "^") {
      toks.push({ t: "op", v: c });
      i++;
      continue;
    }
    if (c === "(") {
      toks.push({ t: "lp", v: c });
      i++;
      continue;
    }
    if (c === ")") {
      toks.push({ t: "rp", v: c });
      i++;
      continue;
    }
    if (c === ",") {
      toks.push({ t: "comma", v: c });
      i++;
      continue;
    }
    throw new Error(`非法字符: ${c}`);
  }
  return toks;
}

const PREC: Record<string, number> = { "+": 1, "-": 1, "*": 2, "/": 2, "%": 2, "^": 3, "u-": 4 };

// 函数变长参数的边界标记（RPN 求值时用于界定 min/max 等变长函数的参数列表）。
const ARG_MARK = Symbol("arg-mark");

function evalExpr(src: string): number {
  const toks = tokenize(src);
  // 插入一元负号标记
  for (let i = 0; i < toks.length; i++) {
    if (toks[i].t === "op" && toks[i].v === "-" && (i === 0 || toks[i - 1].t === "op" || toks[i - 1].t === "lp" || toks[i - 1].t === "comma" || toks[i - 1].t === "fn")) {
      toks[i] = { t: "op", v: "u-" };
    }
  }
  // 调度场 -> RPN
  const out: Tok[] = [];
  const ops: Tok[] = [];
  for (let idx = 0; idx < toks.length; idx++) {
    const tk = toks[idx];
    if (tk.t === "num") out.push(tk);
    else if (tk.t === "fn") ops.push(tk);
    else if (tk.t === "comma") {
      while (ops.length && ops[ops.length - 1].t !== "lp") out.push(ops.pop()!);
    } else if (tk.t === "op") {
      while (
        ops.length &&
        ops[ops.length - 1].t === "op" &&
        PREC[ops[ops.length - 1].v] >= PREC[tk.v]
      ) {
        out.push(ops.pop()!);
      }
      ops.push(tk);
    } else if (tk.t === "lp") {
      // 函数调用左侧的 ( 记为 fopen，供 RPN 求值时界定变长参数
      if (idx > 0 && toks[idx - 1].t === "fn") out.push({ t: "fopen", v: "" });
      ops.push(tk);
    } else if (tk.t === "rp") {
      while (ops.length && ops[ops.length - 1].t !== "lp") out.push(ops.pop()!);
      if (!ops.length) throw new Error("括号不匹配");
      ops.pop(); // lp
      if (ops.length && ops[ops.length - 1].t === "fn") out.push(ops.pop()!);
    }
  }
  while (ops.length) {
    const o = ops.pop()!;
    if (o.t === "lp") throw new Error("括号不匹配");
    out.push(o);
  }
  // RPN 求值
  const st: (number | typeof ARG_MARK)[] = [];
  for (const tk of out) {
    if (tk.t === "num") st.push(parseFloat(tk.v));
    else if (tk.t === "fopen") st.push(ARG_MARK);
    else if (tk.t === "op") {
      if (tk.v === "u-") {
        const a = st.pop();
        if (typeof a !== "number") throw new Error("表达式错误");
        st.push(-a);
      } else {
        const b = st.pop();
        const a = st.pop();
        if (typeof a !== "number" || typeof b !== "number") throw new Error("表达式错误");
        st.push(apply(a, b, tk.v));
      }
    } else if (tk.t === "fn") {
      const args: number[] = [];
      while (st.length && st[st.length - 1] !== ARG_MARK) {
        const v = st.pop();
        if (typeof v !== "number") throw new Error("函数参数不足");
        args.unshift(v);
      }
      if (st.length && st[st.length - 1] === ARG_MARK) st.pop();
      const fn = FUNCS[tk.v];
      if (!fn) throw new Error(`未知函数: ${tk.v}`);
      st.push(fn(args));
    }
  }
  if (st.length !== 1 || typeof st[0] !== "number") throw new Error("表达式错误");
  return st[0];
}

function apply(a: number, b: number, op: string): number {
  switch (op) {
    case "+":
      return a + b;
    case "-":
      return a - b;
    case "*":
      return a * b;
    case "/":
      return a / b;
    case "%":
      return a % b;
    case "^":
      return Math.pow(a, b);
    default:
      throw new Error(`未知运算符: ${op}`);
  }
}

function round(n: number): string {
  if (Number.isInteger(n)) return String(n);
  return String(parseFloat(n.toFixed(6)));
}
