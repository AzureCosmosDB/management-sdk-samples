import process from "node:process";

const MANAGEMENT_SCOPE = "https://management.azure.com/.default";

/**
 * Returns the current principal's Entra object id (OID) without calling Microsoft Graph.
 *
 * Resolution order:
 * 1) `AZURE_PRINCIPAL_OBJECT_ID` (explicit override)
 * 2) `oid` claim from an ARM access token acquired via DefaultAzureCredential
 *
 * @param {import('@azure/core-auth').TokenCredential} credential
 * @returns {Promise<string>}
 */
export async function getCurrentPrincipalObjectId(credential) {
  const override = clean(process.env.AZURE_PRINCIPAL_OBJECT_ID) || clean(process.env.PRINCIPAL_OBJECT_ID);
  if (override) {
    return override;
  }

  const claims = await getArmTokenClaims(credential);
  const oid = claims?.oid;
  if (typeof oid === "string" && oid.trim()) {
    return oid.trim();
  }

  throw new Error(
    "Could not determine current principal object id (oid) from the access token. " +
      "Set AZURE_PRINCIPAL_OBJECT_ID (or PRINCIPAL_OBJECT_ID) to the object id you want to assign roles to.",
  );
}

/**
 * Returns a best-effort user identity string for tagging/ownership metadata.
 *
 * This is intended for logs/tags only; it may be null for service principals / managed identities.
 *
 * @param {import('@azure/core-auth').TokenCredential} credential
 * @returns {Promise<string|null>}
 */
export async function getCurrentUserEmailBestEffort(credential) {
  const claims = await getArmTokenClaims(credential);
  for (const key of ["preferred_username", "upn", "unique_name"]) {
    const value = claims?.[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }

  return null;
}

async function getArmTokenClaims(credential) {
  if (!credential?.getToken) {
    throw new Error("A TokenCredential is required.");
  }

  const token = await credential.getToken(MANAGEMENT_SCOPE);
  const jwt = token?.token;
  if (!jwt || typeof jwt !== "string") {
    throw new Error("Failed to acquire an ARM access token.");
  }

  return parseJwtClaims(jwt);
}

/**
 * Parses the JWT payload into a plain object of claims.
 *
 * This performs a lightweight decode of the middle JWT segment (base64url JSON) and does not
 * validate signatures. It is only used to read non-sensitive claims (like `oid`) from a token
 * already issued by the current credential.
 */
function parseJwtClaims(jwt) {
  const parts = jwt.split(".");
  if (parts.length < 2) {
    throw new Error("Invalid JWT.");
  }

  const payloadJson = bufferFromBase64Url(parts[1]).toString("utf8");
  const parsed = JSON.parse(payloadJson);
  return parsed && typeof parsed === "object" ? parsed : null;
}

function bufferFromBase64Url(base64Url) {
  // base64url -> base64
  let base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
  const padding = base64.length % 4;
  if (padding === 2) base64 += "==";
  else if (padding === 3) base64 += "=";
  else if (padding !== 0) {
    // Some JWT libraries omit padding; if it's otherwise malformed, fail clearly.
    throw new Error("Invalid base64url payload.");
  }

  return Buffer.from(base64, "base64");
}

function clean(value) {
  if (value == null) return null;
  const trimmed = String(value).trim();
  return trimmed ? trimmed : null;
}
