# Usage

```
Usage:  brightctl set 50%
        brightctl set +5%
        brightctl set -5%
        brightctl get
        brightctl restore (to use within a startup script)
```

# Using sudo (not recommended)

You can execute `brightctl` using `sudo` every time. This is the simplest method but is not recommended for security reasons
and is generally inconvenient.

```
sudo brightctl set 50%
```

# Adding udev Rules

The recommended approach is to add your user to the appropriate group (commonly `video`) and create a udev rule. This rule should
automatically grant write permissions for the backlight device to that group.
