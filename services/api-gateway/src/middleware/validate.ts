import type { Request, Response, NextFunction } from 'express';
import type { ZodSchema, ZodError } from 'zod';

function formatZodError(err: ZodError): string {
  return err.errors.map((e) => `${e.path.join('.')}: ${e.message}`).join(', ');
}

/**
 * Returns Express middleware that validates req.body against the given Zod schema.
 * Responds with 400 and a structured error message on validation failure.
 */
export function validateBody<T>(schema: ZodSchema<T>) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const result = schema.safeParse(req.body);
    if (!result.success) {
      res.status(400).json({ error: 'Validation failed', details: formatZodError(result.error) });
      return;
    }
    req.body = result.data;
    next();
  };
}

/**
 * Returns Express middleware that validates req.query against the given Zod schema.
 * Responds with 400 and a structured error message on validation failure.
 */
export function validateQuery<T>(schema: ZodSchema<T>) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const result = schema.safeParse(req.query);
    if (!result.success) {
      res.status(400).json({ error: 'Validation failed', details: formatZodError(result.error) });
      return;
    }
    req.query = result.data as typeof req.query;
    next();
  };
}

/**
 * Returns Express middleware that validates req.params against the given Zod schema.
 * Responds with 400 and a structured error message on validation failure.
 */
export function validateParams<T>(schema: ZodSchema<T>) {
  return (req: Request, res: Response, next: NextFunction): void => {
    const result = schema.safeParse(req.params);
    if (!result.success) {
      res.status(400).json({ error: 'Validation failed', details: formatZodError(result.error) });
      return;
    }
    req.params = result.data as typeof req.params;
    next();
  };
}
