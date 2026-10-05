import jwt from "jsonwebtoken";
import type { Request, Response, NextFunction } from "express";

export interface JwtPayload {
  uid: string;
  role: string;
}

export function authMiddleware(secret: string) {
  return (req: Request, res: Response, next: NextFunction) => {
    const header = req.headers.authorization;
    if (!header || !header.startsWith("Bearer ")) {
      res.status(401).json({ code: 1005, message: "登录状态无效或已过期" });
      return;
    }
    const token = header.slice(7);
    try {
      const payload = jwt.verify(token, secret) as JwtPayload;
      if (!payload.uid || !payload.role) {
        res.status(401).json({ code: 1005, message: "令牌内容无效" });
        return;
      }
      req.userId = payload.uid;
      req.userRole = payload.role;
      req.userToken = token;
      next();
    } catch {
      res.status(401).json({ code: 1005, message: "登录状态无效或已过期" });
    }
  };
}

// 扩展 Express Request 类型，携带解析后的用户信息
declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      userId?: string;
      userRole?: string;
      userToken?: string;
    }
  }
}
