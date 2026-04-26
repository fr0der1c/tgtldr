import { Bootstrap, PendingAuth } from "@/lib/types";
import {
  LoginStage,
  setupSteps,
  SetupStep,
} from "@/components/setup-wizard-types";

export function resolveCurrentStep(
  bootstrap: Bootstrap,
  current: SetupStep,
): SetupStep {
  if (!bootstrap.passwordConfigured) {
    return "password";
  }
  if (current === "bot" && bootstrap.settingsConfigured && bootstrap.telegramAuthorized) {
    return "bot";
  }
  if (current === "login" && bootstrap.settingsConfigured) {
    if (!bootstrap.telegramAuthorized) {
      return "login";
    }
    return "bot";
  }
  if (!bootstrap.settingsConfigured) {
    return "config";
  }
  if (!bootstrap.telegramAuthorized) {
    return "login";
  }
  return "bot";
}

export function resolveLoginStage(
  telegramAuthorized: boolean,
  pendingAuth: PendingAuth | null,
  override: LoginStage | null,
): LoginStage {
  if (override) {
    return override;
  }
  if (telegramAuthorized) {
    return "success";
  }
  if (pendingAuth?.step === "password") {
    return "password";
  }
  if (pendingAuth?.step === "code") {
    return "code";
  }
  return "phone";
}

export function stepEnabled(step: SetupStep, bootstrap: Bootstrap | null) {
  if (step === "password") {
    return !bootstrap || !bootstrap.passwordConfigured;
  }
  if (step === "config") {
    return Boolean(bootstrap?.passwordConfigured && bootstrap.authenticated);
  }
  if (step === "login") {
    return Boolean(
      bootstrap?.passwordConfigured &&
        bootstrap.authenticated &&
        bootstrap.settingsConfigured,
    );
  }
  if (step === "bot") {
    return Boolean(
      bootstrap?.passwordConfigured &&
        bootstrap.authenticated &&
        bootstrap.settingsConfigured &&
        bootstrap.telegramAuthorized,
    );
  }
  return false;
}

export function stepState(
  step: SetupStep,
  currentStep: SetupStep,
  bootstrap: Bootstrap | null,
) {
  if (step === currentStep) {
    return "active";
  }
  if (stepIndex(step) < stepIndex(currentStep)) {
    return "completed";
  }
  if (!stepEnabled(step, bootstrap)) {
    return "locked";
  }
  return "upcoming";
}

export function stepIndex(step: SetupStep) {
  return setupSteps.findIndex((item) => item.key === step);
}

export function asMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
