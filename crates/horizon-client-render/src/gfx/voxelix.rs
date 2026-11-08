//! Voxelix v1.0 configuration.
//!
//! Independent, per-module optimisation flags for Android rendering. Each
//! module can be enabled or disabled separately through environment variables.
//! Hardware capability checks are never bypassed: an enabled flag is only a
//! request, and the renderer falls back to the ordinary path when the device
//! or backend cannot support it.

use std::env;

/// Voxelix v1.0 feature configuration, read once at renderer startup.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct VoxelixConfig {
    /// V01 Multi-Draw Indirect: batch section draws into a single indirect
    /// multi-draw when the GPU and backend support it.
    pub multi_draw: bool,
}

impl VoxelixConfig {
    /// Reads environment variables and returns the effective configuration.
    ///
    /// `HORIZON_VOXELIX_MULTIDRAW=1` opts into V01 on Android; `=0` opts out
    /// off-Android. Absence keeps the platform default: off on Android, on
    /// elsewhere — subject to the hardware capability checks performed by the
    /// renderer (`INDIRECT_FIRST_INSTANCE` + `INDIRECT_EXECUTION`).
    pub fn from_env() -> Self {
        let requested = match env::var("HORIZON_VOXELIX_MULTIDRAW") {
            Ok(value) => {
                let parsed = Self::parse_multi_draw_value(&value);
                let recognized = matches!(
                    value.trim().to_lowercase().as_str(),
                    "1" | "0" | "true" | "false"
                );
                if !recognized {
                    log::warn!(
                        "Voxelix: invalid HORIZON_VOXELIX_MULTIDRAW value {:?}; treating as off",
                        value
                    );
                }
                parsed
            }
            Err(_) => !cfg!(target_os = "android"),
        };

        let log_msg = if cfg!(target_os = "android") {
            if requested {
                "Voxelix V01: Multi-Draw Indirect requested (HORIZON_VOXELIX_MULTIDRAW=1); final use depends on GPU/backend capabilities"
            } else {
                "Voxelix V01: Multi-Draw Indirect disabled by default on Android (set HORIZON_VOXELIX_MULTIDRAW=1 to request)"
            }
        } else if requested {
            "Voxelix V01: Multi-Draw Indirect enabled (subject to GPU/backend capabilities)"
        } else {
            "Voxelix V01: Multi-Draw Indirect explicitly disabled (HORIZON_VOXELIX_MULTIDRAW=0)"
        };
        log::info!("{log_msg}");

        Self { multi_draw: requested }
    }

    /// Parses the raw value of `HORIZON_VOXELIX_MULTIDRAW` into a boolean.
    /// Pure: no environment access, no logging.
    pub(crate) fn parse_multi_draw_value(value: &str) -> bool {
        match value.trim().to_lowercase().as_str() {
            "1" | "true" => true,
            "0" | "false" => false,
            _ => false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_multi_draw_value_1_is_true() {
        assert!(VoxelixConfig::parse_multi_draw_value("1"));
    }

    #[test]
    fn parse_multi_draw_value_true_is_true() {
        assert!(VoxelixConfig::parse_multi_draw_value("true"));
    }

    #[test]
    fn parse_multi_draw_value_0_is_false() {
        assert!(!VoxelixConfig::parse_multi_draw_value("0"));
    }

    #[test]
    fn parse_multi_draw_value_false_is_false() {
        assert!(!VoxelixConfig::parse_multi_draw_value("false"));
    }

    #[test]
    fn parse_multi_draw_value_maybe_is_false() {
        assert!(!VoxelixConfig::parse_multi_draw_value("maybe"));
    }

    #[test]
    fn parse_multi_draw_value_case_insensitive_and_trimmed() {
        assert!(VoxelixConfig::parse_multi_draw_value("TRUE"));
        assert!(VoxelixConfig::parse_multi_draw_value(" True "));
        assert!(!VoxelixConfig::parse_multi_draw_value("FALSE"));
        assert!(!VoxelixConfig::parse_multi_draw_value(" 0 "));
    }
}
