# Chabashira

Simple bridge tool between [genmai](https://github.com/naoina/genmai) and [ridgepole](https://github.com/winebarrel/ridgepole).

## Dependency

* [naoina/go-stringutil](https://github.com/naoina/go-stringutil.git)

## Installlation

```
go install github.com/Talos208/chabashira
```

## Schema

It's based on schema description of [genmai](https://github.com/naoina/genmai).

```go
// It isn't target.
type Hoge struct {
  ...
}

// It is target.
// db:"entity"
type Fuga struct {
  Id int64 `db:"pk"`       // Primary Key
  A  sql.NullInt64         // Nullable integer
  B  string `db:"unique"`  // varchar with Unique constraint
  C  string `limit:"30"`   // varchar(30)
  D  float `default:"1.0"` // Real with default value
  E  []byte                // Binary
  F  time.Time             // timestamp
  PiyoId int64 `refer:""`  // Reference to table "Piyo"
}

// It is target too.
// db:"entity"
type Piyo struct {
  Id int64 `db:"pk"`
  ...
}

```

## Usage

```
chabashira <directory or file>
```

If specify file, process it. If specify path, process all .go files in sepcified directory.
Chabashira collect schema information from struct which hava `db:"entity"` in it's comment. Then generates Rails's migration format file. Like this.

```ruby
create_table 'fuga' do |t|
  t.integer :a
  t.string :b, unique:true, null:false
  t.string :c, null:false
  t.float :d, null:false
  t.binary :e
  t.timestamp :f, null:false
  t.references :piyo
end

create_table 'piyo' do |t|
  ...
end
```
