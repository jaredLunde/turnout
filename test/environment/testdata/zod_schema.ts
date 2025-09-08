import { z } from "zod";

export const serverSchema = z.object({
  /**
   * The node environment the server is running in. Should be "production"
   * for both staging and production deployment environments.
   */
  NODE_ENV: z
    .enum(["development", "test", "production"])
    .default("development"),

  /**
   * The hostname the server should listen on
   */
  HOSTNAME: z.string().default("0.0.0.0"),

  /**
   * The port the server should listen on
   */
  PORT: z.coerce.number().int().positive().default(4000),

  /**
   * A publicly accessible URL for the server. This is used for generating
   * absolute URLs for emails and other things.
   */
  RAILWAY_PUBLIC_DOMAIN: z.string().default("localhost:4000"),

  /**
   * The URL of the dashboard
   */
  DASHBOARD_URL: z.string().url().default("http://localhost:3000"),
});