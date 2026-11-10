-- AlterTable
ALTER TABLE "Project" ADD COLUMN     "slug" TEXT,
ADD COLUMN     "category" TEXT NOT NULL DEFAULT 'web',
ADD COLUMN     "status" TEXT NOT NULL DEFAULT 'live',
ADD COLUMN     "year" TEXT,
ADD COLUMN     "role" TEXT,
ADD COLUMN     "highlights" TEXT[] DEFAULT ARRAY[]::TEXT[],
ADD COLUMN     "releaseUrl" TEXT,
ADD COLUMN     "featured" BOOLEAN NOT NULL DEFAULT false;

-- Backfill slugs from titles with the same rule as slugify() in src/lib/project.ts;
-- duplicate titles get -2, -3 in id order.
WITH base AS (
    SELECT "id", trim(both '-' from regexp_replace(lower("title"), '[^a-z0-9]+', '-', 'g')) AS "s"
    FROM "Project"
), ranked AS (
    SELECT "id", "s", row_number() OVER (PARTITION BY "s" ORDER BY "id") AS "n"
    FROM base
    WHERE "s" <> ''
)
UPDATE "Project" p
SET "slug" = CASE WHEN r."n" = 1 THEN r."s" ELSE r."s" || '-' || r."n" END
FROM ranked r
WHERE p."id" = r."id";

-- CreateIndex
CREATE UNIQUE INDEX "Project_slug_key" ON "Project"("slug");
