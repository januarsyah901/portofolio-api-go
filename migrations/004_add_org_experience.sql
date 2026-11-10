-- CreateTable
CREATE TABLE "OrgExperience" (
    "id" SERIAL NOT NULL,
    "period" TEXT NOT NULL,
    "organization" TEXT NOT NULL,
    "role" TEXT NOT NULL,
    "description" TEXT NOT NULL DEFAULT '',
    "highlights" TEXT[],
    "type" TEXT NOT NULL DEFAULT 'organization',
    "logo" TEXT,
    "order" INTEGER NOT NULL DEFAULT 0,
    "visible" BOOLEAN NOT NULL DEFAULT true,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "OrgExperience_pkey" PRIMARY KEY ("id")
);

