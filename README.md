# speed
Simple performance monitor command for reading Linux file.

## Example
![speed sample](https://github.com/tesujiro/tesujiro/Speed_Screenshot_20190715_01.gif "speed")

## Requirement
Linux or MacOSX

## Usage
### Show speed of Stdin
```
> SomeCommand | speed | SomeNextCommand
```

### Limit bandwidth
```
> SomeCommand | speed -b 10MB | SomeNextCommand
```

### Show progress
```
> speed -g SomeBigFile | SomeNextCommand
```

### Show progress widh bandwidth
```
> speed -g -b 10MB SomeBigFile | SomeNextCommand
```

## VS. 
pv: 

## Install
```
> go get github.com/tesujiro/speed
```

## Licence

[MIT](https://github.com/tcnksm/tool/blob/master/LICENCE)

## Author

[tesujiro](https://github.com/tesujiro)

